package tasks

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-co-op/gocron"
	fastshot "github.com/opus-domini/fast-shot"
	"github.com/paycrest/protocol/config"
	"github.com/paycrest/protocol/ent"
	"github.com/paycrest/protocol/ent/fiatcurrency"
	"github.com/paycrest/protocol/ent/lockorderfulfillment"
	"github.com/paycrest/protocol/ent/lockpaymentorder"
	networkent "github.com/paycrest/protocol/ent/network"
	"github.com/paycrest/protocol/ent/paymentorder"
	"github.com/paycrest/protocol/ent/providerordertoken"
	"github.com/paycrest/protocol/ent/receiveaddress"
	"github.com/paycrest/protocol/ent/token"
	"github.com/paycrest/protocol/ent/webhookretryattempt"
	"github.com/paycrest/protocol/services"
	orderService "github.com/paycrest/protocol/services/order"
	"github.com/paycrest/protocol/storage"
	"github.com/paycrest/protocol/types"
	"github.com/paycrest/protocol/utils"
	"github.com/paycrest/protocol/utils/logger"
	"github.com/shopspring/decimal"
)

var orderConf = config.OrderConfig()
var serverConf = config.ServerConfig()

var rpcClients = map[string]types.RPCClient{}

// setRPCClients connects to the RPC endpoints of all networks
func setRPCClients(ctx context.Context) ([]*ent.Network, error) {
	isTestnet := false
	if serverConf.Environment != "production" {
		isTestnet = true
	}

	networks, err := storage.Client.Network.
		Query().
		Where(networkent.IsTestnetEQ(isTestnet)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("setRPCClients.fetchNetworks: %w", err)
	}

	// Connect to RPC endpoint
	var client types.RPCClient
	for _, network := range networks {
		if rpcClients[network.Identifier] == nil && !strings.HasPrefix(network.Identifier, "tron") {
			retryErr := utils.Retry(3, 1*time.Second, func() error {
				client, err = types.NewEthClient(network.RPCEndpoint)
				return err
			})
			if retryErr != nil {
				logger.Errorf("failed to connect to %s RPC %v", network.Identifier, retryErr)
				continue
			}

			rpcClients[network.Identifier] = client
		}
	}

	return networks, nil
}

// RetryStaleUserOperations retries stale user operations
func RetryStaleUserOperations() error {
	ctx := context.Background()
	var wg sync.WaitGroup

	// Establish RPC connections
	_, err := setRPCClients(ctx)
	if err != nil {
		return fmt.Errorf("RetryStaleUserOperations: %w", err)
	}

	// Process initiated orders
	orders, err := storage.Client.PaymentOrder.
		Query().
		Where(func(s *sql.Selector) {
			ra := sql.Table(receiveaddress.Table)
			s.LeftJoin(ra).On(s.C(paymentorder.FieldReceiveAddressText), ra.C(receiveaddress.FieldAddress)).
				Where(sql.And(
					sql.EQ(s.C(paymentorder.FieldStatus), paymentorder.StatusInitiated),
					sql.EQ(ra.C(receiveaddress.FieldStatus), receiveaddress.StatusUsed),
					sql.GTE(s.C(paymentorder.FieldUpdatedAt), time.Now().Add(-10*time.Minute)),
					sql.IsNull(s.C(paymentorder.FieldGatewayID)),
				))
		}).
		WithToken(func(tq *ent.TokenQuery) {
			tq.WithNetwork()
		}).
		All(ctx)
	if err != nil {
		return fmt.Errorf("RetryStaleUserOperations: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, order := range orders {
			orderAmountWithFees := order.Amount.Add(order.NetworkFee).Add(order.SenderFee).Add(order.ProtocolFee)
			if order.AmountPaid.GreaterThanOrEqual(orderAmountWithFees) {
				var service types.OrderService
				if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
					service = orderService.NewOrderTron()
				} else {
					service = orderService.NewOrderEVM()
				}
				err := service.CreateOrder(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order.ID)
				if err != nil {
					logger.Errorf("process task to create orders => %v", err)
				}
			}
		}
	}()

	// Revert order process
	orders, err = storage.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.Or(
				paymentorder.StatusEQ(paymentorder.StatusInitiated),
				paymentorder.StatusEQ(paymentorder.StatusExpired),
			),
			paymentorder.AmountPaidGT(decimal.Zero),
			paymentorder.UpdatedAtLT(time.Now().Add(-10*time.Minute)),
		).
		WithReceiveAddress().
		WithRecipient().
		WithToken(func(tq *ent.TokenQuery) {
			tq.WithNetwork()
		}).
		All(ctx)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, order := range orders {
			if order.Edges.ReceiveAddress.Status == receiveaddress.StatusExpired || order.Edges.ReceiveAddress.Status == receiveaddress.StatusUsed {
				var service types.OrderService
				if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
					service = orderService.NewOrderTron()
				} else {
					service = orderService.NewOrderEVM()
				}
				err := service.RevertOrder(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order)
				if err != nil {
					logger.Errorf("process task to revert orders => %v", err)
				}
			}
		}
	}()

	// Settle order process
	lockOrders, err := storage.Client.LockPaymentOrder.
		Query().
		Where(
			lockpaymentorder.StatusEQ(lockpaymentorder.StatusValidated),
			lockpaymentorder.HasFulfillmentWith(
				lockorderfulfillment.ValidationStatusEQ(lockorderfulfillment.ValidationStatusSuccess),
			),
			lockpaymentorder.UpdatedAtLT(time.Now().Add(-5*time.Minute)),
		).
		WithToken(func(tq *ent.TokenQuery) {
			tq.WithNetwork()
		}).
		All(ctx)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, order := range lockOrders {
			var service types.OrderService
			if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
				service = orderService.NewOrderTron()
			} else {
				service = orderService.NewOrderEVM()
			}
			err := service.SettleOrder(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order.ID)
			if err != nil {
				logger.Errorf("process order settlements task => %v", err)
			}
		}
	}()

	// Refund order process
	lockOrders, err = storage.Client.LockPaymentOrder.
		Query().
		Where(
			lockpaymentorder.GatewayIDNEQ(""),
			lockpaymentorder.Or(
				lockpaymentorder.StatusEQ(lockpaymentorder.StatusPending),
				lockpaymentorder.StatusEQ(lockpaymentorder.StatusCancelled),
			),
			lockpaymentorder.CreatedAtLTE(time.Now().Add(-30*time.Minute)),
		).
		WithToken(func(tq *ent.TokenQuery) {
			tq.WithNetwork()
		}).
		All(ctx)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, order := range lockOrders {
			var service types.OrderService
			if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
				service = orderService.NewOrderTron()
			} else {
				service = orderService.NewOrderEVM()
			}
			err := service.RefundOrder(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order.GatewayID)
			if err != nil {
				logger.Errorf("process order refunds task => %v", err)
			}
		}
	}()

	return nil
}

// IndexBlockchainEvents indexes missed blocks
func IndexBlockchainEvents() error {
	ctx := context.Background()
	var wg sync.WaitGroup

	time.Sleep(100 * time.Millisecond) // to keep out of sync with other tasks

	// Establish RPC connections
	networks, err := setRPCClients(ctx)
	if err != nil {
		return fmt.Errorf("IndexBlockchainEvents: %w", err)
	}

	// Index ERC20 transfer events
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = utils.Retry(8, 2*time.Second, func() error {
			orders, err := storage.Client.PaymentOrder.
				Query().
				Where(
					paymentorder.StatusEQ(paymentorder.StatusInitiated),
					paymentorder.HasReceiveAddressWith(
						receiveaddress.Or(
							receiveaddress.StatusEQ(receiveaddress.StatusUnused),
							receiveaddress.StatusEQ(receiveaddress.StatusPartial),
						),
						receiveaddress.ValidUntilGT(time.Now()),
					),
				).
				WithToken(func(tq *ent.TokenQuery) {
					tq.WithNetwork()
				}).
				WithReceiveAddress().
				WithRecipient().
				All(ctx)
			if err != nil {
				logger.Errorf("IndexBlockchainEvents: %v", err)
			}

			if len(orders) > 0 {
				for _, order := range orders {
					if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
						indexerService := services.NewIndexerService(orderService.NewOrderTron())
						err := indexerService.IndexTRC20Transfer(ctx, order)
						if err != nil {
							continue
						}
					} else {
						indexerService := services.NewIndexerService(orderService.NewOrderEVM())
						err := indexerService.IndexERC20Transfer(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order)
						if err != nil {
							continue
						}
					}
				}
			}

			return fmt.Errorf("trigger retry")
		})
	}()

	// Index OrderCreated events
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond)
		_ = utils.Retry(8, 2*time.Second, func() error {
			for _, network := range networks {
				// Index events triggered from API
				orders, err := storage.Client.PaymentOrder.
					Query().
					Where(func(s *sql.Selector) {
						lpo := sql.Table(lockpaymentorder.Table)
						s.Where(sql.And(
							sql.EQ(s.C(paymentorder.FieldStatus), paymentorder.StatusPending),
							sql.Or(
								sql.NotExists(
									sql.Select().
										From(lpo).
										Where(sql.ColumnsEQ(s.C(paymentorder.FieldGatewayID), lpo.C(lockpaymentorder.FieldGatewayID))),
								),
								sql.IsNull(s.C(paymentorder.FieldGatewayID)),
							),
							sql.GT(s.C(paymentorder.FieldBlockNumber), 0),
						))
					}).
					Where(paymentorder.HasTokenWith(token.HasNetworkWith(networkent.IDEQ(network.ID)))).
					WithReceiveAddress().
					WithToken(func(tq *ent.TokenQuery) {
						tq.WithNetwork()
					}).
					Order(ent.Asc(paymentorder.FieldBlockNumber)).
					All(ctx)
				if err != nil {
					continue
				}

				if len(orders) > 0 {
					for _, order := range orders {
						if strings.HasPrefix(network.Identifier, "tron") {
							indexerService := services.NewIndexerService(orderService.NewOrderTron())
							err := indexerService.IndexOrderCreatedTron(ctx, order)
							if err != nil {
								continue
							}
						} else {
							indexerService := services.NewIndexerService(orderService.NewOrderEVM())
							err := indexerService.IndexOrderCreated(ctx, rpcClients[network.Identifier], network, order.Edges.ReceiveAddress.Address)
							if err != nil {
								continue
							}
						}
					}
				}

				// Index events triggered from Gateway contract
				if !strings.HasPrefix(network.Identifier, "tron") {
					indexerService := services.NewIndexerService(orderService.NewOrderEVM())
					err = indexerService.IndexOrderCreated(ctx, rpcClients[network.Identifier], network, "")
					if err != nil {
						continue
					}
				}
			}

			return fmt.Errorf("trigger retry")
		})
	}()

	// Index OrderSettled events
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1000 * time.Millisecond)
		_ = utils.Retry(3, 2*time.Second, func() error {
			lockOrders, err := storage.Client.LockPaymentOrder.
				Query().
				Where(func(s *sql.Selector) {
					po := sql.Table(paymentorder.Table)
					s.LeftJoin(po).On(s.C(lockpaymentorder.FieldGatewayID), po.C(paymentorder.FieldGatewayID)).
						Where(sql.Or(
							sql.EQ(s.C(lockpaymentorder.FieldStatus), lockpaymentorder.StatusValidated),
							sql.And(
								sql.EQ(po.C(paymentorder.FieldStatus), paymentorder.StatusPending),
								sql.EQ(s.C(lockpaymentorder.FieldStatus), lockpaymentorder.StatusSettled),
							)),
						)
				}).
				Where(lockpaymentorder.GatewayIDNEQ("")). // Exclude orders without gateway ID
				WithToken(func(tq *ent.TokenQuery) {
					tq.WithNetwork()
				}).
				Order(ent.Asc(lockpaymentorder.FieldBlockNumber)).
				All(ctx)
			if err != nil {
				logger.Errorf("IndexBlockchainEvents: %v", err)
			}

			if len(lockOrders) > 0 {
				for _, order := range lockOrders {
					// Fetch payment order
					paymentOrderExists := true
					paymentOrder, err := storage.Client.PaymentOrder.
						Query().
						Where(
							paymentorder.GatewayIDEQ(order.GatewayID),
							paymentorder.StatusEQ(paymentorder.StatusPending),
						).
						Only(ctx)
					if err != nil {
						if ent.IsNotFound(err) {
							// Payment order does not exist, no need to update
							paymentOrderExists = false
						}
					}

					// Update payment order status for settled lock order
					if paymentOrderExists {
						settledPercent := paymentOrder.PercentSettled.Add(order.OrderPercent)

						if settledPercent.GreaterThanOrEqual(decimal.NewFromInt(100)) {
							_, err = storage.Client.PaymentOrder.
								Update().
								SetBlockNumber(order.BlockNumber).
								SetTxHash(order.TxHash).
								SetPercentSettled(settledPercent).
								SetStatus(paymentorder.StatusSettled).
								Save(ctx)
							if err != nil {
								logger.Errorf("IndexBlockchainEvents: %v", err)
							}
						}
					}

					if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
						indexerService := services.NewIndexerService(orderService.NewOrderTron())
						err := indexerService.IndexOrderSettledTron(ctx, order)
						if err != nil {
							continue
						}
					} else {
						indexerService := services.NewIndexerService(orderService.NewOrderEVM())
						err := indexerService.IndexOrderSettled(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order.Edges.Token.Edges.Network, order.GatewayID)
						if err != nil {
							continue
						}
					}
				}
			}

			return fmt.Errorf("trigger retry")
		})
	}()

	// Index OrderRefunded events
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1500 * time.Millisecond)
		_ = utils.Retry(3, 2*time.Second, func() error {
			lockOrders, err := storage.Client.LockPaymentOrder.
				Query().
				Where(func(s *sql.Selector) {
					po := sql.Table(paymentorder.Table)
					s.LeftJoin(po).On(s.C(lockpaymentorder.FieldGatewayID), po.C(paymentorder.FieldGatewayID)).
						Where(sql.Or(
							sql.And(
								sql.EQ(s.C(lockpaymentorder.FieldStatus), lockpaymentorder.StatusPending),
								sql.LT(s.C(lockpaymentorder.FieldCreatedAt), time.Now().Add(-35*time.Minute)),
							),
							sql.And(
								sql.EQ(po.C(paymentorder.FieldStatus), paymentorder.StatusPending),
								sql.EQ(s.C(lockpaymentorder.FieldStatus), lockpaymentorder.StatusRefunded),
							),
						))
				}).
				WithToken(func(tq *ent.TokenQuery) {
					tq.WithNetwork()
				}).
				Order(ent.Asc(lockpaymentorder.FieldBlockNumber)).
				All(ctx)
			if err != nil {
				logger.Errorf("IndexBlockchainEvents: %v", err)
			}

			if len(lockOrders) > 0 {
				for _, order := range lockOrders {
					if strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
						indexerService := services.NewIndexerService(orderService.NewOrderTron())
						err := indexerService.IndexOrderRefundedTron(ctx, order)
						if err != nil {
							continue
						}
					} else {
						indexerService := services.NewIndexerService(orderService.NewOrderEVM())
						err := indexerService.IndexOrderRefunded(ctx, rpcClients[order.Edges.Token.Edges.Network.Identifier], order.Edges.Token.Edges.Network, order.GatewayID)
						if err != nil {
							continue
						}
					}
				}
			}

			return fmt.Errorf("trigger retry")
		})
	}()

	return nil
}

// HandleReceiveAddressValidity handles receive address validity
func HandleReceiveAddressValidity() error {
	ctx := context.Background()

	// Establish RPC connections
	_, err := setRPCClients(ctx)
	if err != nil {
		return fmt.Errorf("HandleReceiveAddressValidity: %w", err)
	}

	// Fetch expired receive addresses that are due for validity check
	addresses, err := storage.Client.ReceiveAddress.
		Query().
		Where(
			receiveaddress.ValidUntilLTE(time.Now()),
			receiveaddress.Or(
				receiveaddress.StatusNEQ(receiveaddress.StatusUsed),
				receiveaddress.And(
					receiveaddress.StatusEQ(receiveaddress.StatusUsed),
					receiveaddress.HasPaymentOrderWith(
						paymentorder.StatusEQ(paymentorder.StatusInitiated),
					),
				),
			),
			receiveaddress.HasPaymentOrder(),
		).
		WithPaymentOrder(func(po *ent.PaymentOrderQuery) {
			po.WithToken(func(tq *ent.TokenQuery) {
				tq.WithNetwork()
			})
			po.WithRecipient()
		}).
		All(ctx)
	if err != nil {
		return fmt.Errorf("HandleReceiveAddressValidity: %w", err)
	}

	var indexerService services.Indexer
	for _, address := range addresses {
		if strings.HasPrefix(address.Edges.PaymentOrder.Edges.Token.Edges.Network.Identifier, "tron") {
			indexerService = services.NewIndexerService(orderService.NewOrderTron())
		} else {
			indexerService = services.NewIndexerService(orderService.NewOrderEVM())
		}

		err := indexerService.HandleReceiveAddressValidity(ctx, rpcClients[address.Edges.PaymentOrder.Edges.Token.Edges.Network.Identifier], address, address.Edges.PaymentOrder)
		if err != nil {
			continue
		}
	}

	return nil
}

// SubscribeToRedisKeyspaceEvents subscribes to redis keyspace events according to redis.conf settings
func SubscribeToRedisKeyspaceEvents() {
	ctx := context.Background()

	// Handle expired or deleted order request key events
	orderRequest := storage.RedisClient.PSubscribe(
		ctx,
		"__keyevent@0__:expired:order_request_*",
		"__keyevent@0__:del:order_request_*",
	)
	orderRequestChan := orderRequest.Channel()

	go services.NewPriorityQueueService().ReassignStaleOrderRequest(ctx, orderRequestChan)
}

// fetchExternalRate fetches the external rate for a fiat currency
func fetchExternalRate(currency string) (decimal.Decimal, error) {
	// Fetch stable coin rate from third-party API Quidax (USDT)
	res, err := fastshot.NewClient("https://www.quidax.com").
		Config().SetTimeout(30*time.Second).
		Build().GET(fmt.Sprintf("/api/v1/markets/tickers/usdt%s", strings.ToLower(currency))).
		Retry().Set(3, 5*time.Second).
		Send()
	if err != nil {
		return decimal.Zero, fmt.Errorf("ComputeMarketRate: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return decimal.Zero, fmt.Errorf("ComputeMarketRate: %w", err)
	}

	price, err := decimal.NewFromString(data["data"].(map[string]interface{})["ticker"].(map[string]interface{})["buy"].(string))
	if err != nil {
		return decimal.Zero, fmt.Errorf("ComputeMarketRate: %w", err)
	}

	return price, nil
}

// ComputeMarketRate computes the market price for fiat currencies
func ComputeMarketRate() error {
	ctx := context.Background()

	// Fetch all fiat currencies
	currencies, err := storage.Client.FiatCurrency.
		Query().
		Where(fiatcurrency.IsEnabledEQ(true)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("ComputeMarketRate: %w", err)
	}

	for _, currency := range currencies {
		// Fetch external rate
		externalRate, err := fetchExternalRate(currency.Code)
		if err != nil {
			return fmt.Errorf("ComputeMarketRate: %w", err)
		}

		// Fetch rates from token configs with fixed conversion rate
		tokenConfigs, err := storage.Client.ProviderOrderToken.
			Query().
			Where(
				providerordertoken.SymbolIn("USDT", "USDC"),
				providerordertoken.ConversionRateTypeEQ(providerordertoken.ConversionRateTypeFixed),
			).
			Select(providerordertoken.FieldFixedConversionRate).
			All(ctx)
		if err != nil {
			return fmt.Errorf("ComputeMarketRate: %w", err)
		}

		var rates []decimal.Decimal
		for _, tokenConfig := range tokenConfigs {
			rates = append(rates, tokenConfig.FixedConversionRate)
		}

		// Calculate median
		median := utils.Median(rates)

		// Check the median rate against the external rate to ensure it's not too far off
		percentDeviation := utils.AbsPercentageDeviation(externalRate, median)
		if percentDeviation.GreaterThan(orderConf.PercentDeviationFromExternalRate) {
			median = externalRate
		}

		// Update currency with median rate
		_, err = storage.Client.FiatCurrency.
			UpdateOneID(currency.ID).
			SetMarketRate(median).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("ComputeMarketRate: %w", err)
		}
	}

	return nil
}

// Retry failed webhook notifications
func RetryFailedWebhookNotifications() error {
	ctx := context.Background()

	// Fetch failed webhook notifications that are due for retry
	attempts, err := storage.Client.WebhookRetryAttempt.
		Query().
		Where(
			webhookretryattempt.StatusEQ(webhookretryattempt.StatusFailed),
			webhookretryattempt.NextRetryTimeLTE(time.Now()),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("RetryFailedWebhookNotifications: %w", err)
	}

	baseDelay := 2 * time.Minute
	maxCumulativeTime := 24 * time.Hour

	for _, attempt := range attempts {
		// Send the webhook notification
		_, err = fastshot.NewClient(attempt.WebhookURL).
			Config().SetTimeout(30*time.Second).
			Header().Add("X-Paycrest-Signature", attempt.Signature).
			Build().POST("").
			Body().AsJSON(attempt.Payload).
			Send()
		if err != nil {
			// Webhook notification failed
			// Update attempt with next retry time
			attemptNumber := attempt.AttemptNumber + 1
			delay := baseDelay * time.Duration(math.Pow(2, float64(attemptNumber-1)))

			nextRetryTime := time.Now().Add(delay)

			attemptUpdate := attempt.Update()

			attemptUpdate.
				AddAttemptNumber(1).
				SetNextRetryTime(nextRetryTime)

			// Set status to expired if cumulative time is greater than 24 hours
			if nextRetryTime.Sub(attempt.CreatedAt.Add(-baseDelay)) > maxCumulativeTime {
				attemptUpdate.SetStatus(webhookretryattempt.StatusExpired)
			}

			_, err := attemptUpdate.Save(ctx)
			if err != nil {
				return fmt.Errorf("RetryFailedWebhookNotifications: %w", err)
			}

			continue
		}

		// Webhook notification was successful
		_, err := attempt.Update().
			SetStatus(webhookretryattempt.StatusSuccess).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("RetryFailedWebhookNotifications: %w", err)
		}
	}

	return nil
}

// StartCronJobs starts cron jobs
func StartCronJobs() {
	serverConf := config.ServerConfig()
	scheduler := gocron.NewScheduler(time.UTC)
	priorityQueue := services.NewPriorityQueueService()

	if serverConf.Environment != "production" {
		err := ComputeMarketRate()
		if err != nil {
			logger.Errorf("StartCronJobs: %v", err)
		}

		err = priorityQueue.ProcessBucketQueues()
		if err != nil {
			logger.Errorf("StartCronJobs: %v", err)
		}
	}

	// Compute market rate every 4 minutes
	_, err := scheduler.Cron("*/4 * * * *").Do(ComputeMarketRate)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Refresh provision bucket priority queues every X minutes
	_, err = scheduler.Cron(fmt.Sprintf("*/%d * * * *", orderConf.BucketQueueRebuildInterval)).
		Do(priorityQueue.ProcessBucketQueues)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Retry failed webhook notifications every 60 minutes
	_, err = scheduler.Cron("*/60 * * * *").Do(RetryFailedWebhookNotifications)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Reassign pending order requests every 13 minutes
	_, err = scheduler.Cron("*/13 * * * *").Do(priorityQueue.ReassignPendingOrders)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Reassign unvalidated order requests every 11 minutes
	_, err = scheduler.Cron("*/11 * * * *").Do(priorityQueue.ReassignUnvalidatedLockOrders)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Reassign unfulfilled order requests every 10 minutes
	_, err = scheduler.Cron("*/10 * * * *").Do(priorityQueue.ReassignUnfulfilledLockOrders)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Handle receive address validity every 31 minutes
	_, err = scheduler.Cron("*/31 * * * *").Do(HandleReceiveAddressValidity)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Retry stale user operations every 2 minutes
	_, err = scheduler.Cron("*/2 * * * *").Do(RetryStaleUserOperations)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Index blockchain events every 1 minute
	_, err = scheduler.Cron("*/1 * * * *").Do(IndexBlockchainEvents)
	if err != nil {
		logger.Errorf("StartCronJobs: %v", err)
	}

	// Start scheduler
	scheduler.StartAsync()
}
