package services

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
	fastshot "github.com/opus-domini/fast-shot"
	"github.com/paycrest/protocol/config"
	"github.com/paycrest/protocol/ent"
	"github.com/paycrest/protocol/ent/fiatcurrency"
	"github.com/paycrest/protocol/ent/institution"
	"github.com/paycrest/protocol/ent/lockpaymentorder"
	networkent "github.com/paycrest/protocol/ent/network"
	"github.com/paycrest/protocol/ent/paymentorder"
	"github.com/paycrest/protocol/ent/providerprofile"
	"github.com/paycrest/protocol/ent/provisionbucket"
	"github.com/paycrest/protocol/ent/receiveaddress"
	"github.com/paycrest/protocol/ent/senderprofile"
	"github.com/paycrest/protocol/ent/token"
	"github.com/paycrest/protocol/ent/transactionlog"
	"github.com/paycrest/protocol/ent/user"
	"github.com/paycrest/protocol/services/contracts"
	db "github.com/paycrest/protocol/storage"
	"github.com/paycrest/protocol/types"
	"github.com/paycrest/protocol/utils"
	cryptoUtils "github.com/paycrest/protocol/utils/crypto"
	"github.com/paycrest/protocol/utils/logger"
	"github.com/shopspring/decimal"
)

var orderConf = config.OrderConfig()
var serverConf = config.ServerConfig()

// Indexer is an interface for indexing blockchain data to the database.
type Indexer interface {
	IndexERC20Transfer(ctx context.Context, client types.RPCClient, order *ent.PaymentOrder) error
	IndexTRC20Transfer(ctx context.Context, order *ent.PaymentOrder) error
	IndexOrderCreated(ctx context.Context, client types.RPCClient, network *ent.Network, sender string) error
	IndexOrderCreatedTron(ctx context.Context, order *ent.PaymentOrder) error
	IndexOrderSettled(ctx context.Context, client types.RPCClient, network *ent.Network, gatewayId string) error
	IndexOrderSettledTron(ctx context.Context, order *ent.LockPaymentOrder) error
	IndexOrderRefunded(ctx context.Context, client types.RPCClient, network *ent.Network, gatewayId string) error
	IndexOrderRefundedTron(ctx context.Context, order *ent.LockPaymentOrder) error
	HandleReceiveAddressValidity(ctx context.Context, client types.RPCClient, receiveAddress *ent.ReceiveAddress, paymentOrder *ent.PaymentOrder) error
	CreateLockPaymentOrder(ctx context.Context, client types.RPCClient, network *ent.Network, event *types.OrderCreatedEvent) error
	UpdateReceiveAddressStatus(ctx context.Context, client types.RPCClient, receiveAddress *ent.ReceiveAddress, paymentOrder *ent.PaymentOrder, event *types.TokenTransferEvent) (bool, error)
	UpdateOrderStatusSettled(ctx context.Context, event *types.OrderSettledEvent) error
	UpdateOrderStatusRefunded(ctx context.Context, event *types.OrderRefundedEvent) error
}

// IndexerService performs blockchain to database extract, transform, load (ETL) operations.
type IndexerService struct {
	priorityQueue *PriorityQueueService
	order         types.OrderService
}

// NewIndexerService creates a new instance of IndexerService.
func NewIndexerService(order types.OrderService) Indexer {
	priorityQueue := NewPriorityQueueService()

	return &IndexerService{
		priorityQueue: priorityQueue,
		order:         order,
	}
}

// IndexERC20Transfer indexes transfers to the receive address for an EVM network.
func (s *IndexerService) IndexERC20Transfer(ctx context.Context, client types.RPCClient, order *ent.PaymentOrder) error {
	var err error

	// Connect to RPC endpoint
	retryErr := utils.Retry(3, 1*time.Second, func() error {
		client, err = types.NewEthClient(order.Edges.Token.Edges.Network.RPCEndpoint)
		return err
	})
	if retryErr != nil {
		return retryErr
	}

	// Initialize contract filterer
	filterer, err := contracts.NewERC20TokenFilterer(common.HexToAddress(order.Edges.Token.ContractAddress), client)
	if err != nil {
		logger.Errorf("IndexERC20Transfer.NewERC20TokenFilterer: %v", err)
		return err
	}

	// Fetch current block header
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		logger.Errorf("IndexERC20Transfer.HeaderByNumber: %v", err)
	}
	toBlock := header.Number.Uint64()

	// Fetch logs
	var iter *contracts.ERC20TokenTransferIterator
	retryErr = utils.Retry(3, 1*time.Second, func() error {
		var err error
		iter, err = filterer.FilterTransfer(&bind.FilterOpts{
			Start: uint64(int64(toBlock) - 5000),
			End:   &toBlock,
		}, nil, []common.Address{common.HexToAddress(order.Edges.ReceiveAddress.Address)})
		return err
	})
	if retryErr != nil {
		logger.Errorf("IndexERC20Transfer.FilterTransfer: %v", retryErr)
		return retryErr
	}

	// Iterate over logs
	for iter.Next() {
		transferEvent := &types.TokenTransferEvent{
			BlockNumber: iter.Event.Raw.BlockNumber,
			TxHash:      iter.Event.Raw.TxHash.Hex(),
			From:        iter.Event.From.Hex(),
			To:          iter.Event.To.Hex(),
			Value:       iter.Event.Value,
		}

		ok, err := s.UpdateReceiveAddressStatus(ctx, client, order.Edges.ReceiveAddress, order, transferEvent)
		if err != nil {
			logger.Errorf("IndexERC20Transfer.UpdateReceiveAddressStatus: %v", err)
			continue
		}
		if ok {
			return err
		}
	}

	return nil
}

// IndexTRC20Transfer indexes transfers to the receive address for Tron network.
func (s *IndexerService) IndexTRC20Transfer(ctx context.Context, order *ent.PaymentOrder) error {
	var err error

	if !strings.HasPrefix(order.Edges.Token.Edges.Network.Identifier, "tron") {
		return fmt.Errorf("IndexTRC20Transfer: invalid network identifier: %s", order.Edges.Token.Edges.Network.Identifier)
	}

	client := fastshot.NewClient(order.Edges.Token.Edges.Network.RPCEndpoint).
		Config().SetTimeout(30*time.Second).
		Header().Add("TRON_PRO_API_KEY", orderConf.TronProApiKey)

	// TODO: should we include '?only_confirmed=true' in the URL?
	res, err := client.Build().
		GET(fmt.Sprintf("/v1/accounts/%s/transactions/trc20", order.Edges.ReceiveAddress.Address)).
		Retry().Set(3, 1*time.Second).
		Send()
	if err != nil {
		logger.Errorf("IndexTRC20Transfer.FetchTransfer: %v", err)
		return err
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		logger.Errorf("IndexTRC20Transfer.ParseJSONResponse: %v", err)
		return err
	}

	if data["success"].(bool) {
		for _, event := range data["data"].([]interface{}) {
			eventData := event.(map[string]interface{})

			value, err := decimal.NewFromString(eventData["value"].(string))
			if err != nil {
				logger.Errorf("IndexTRC20Transfer.NewFromString: %v", err)
				return err
			}

			amountPaid := utils.FromSubunit(value.BigInt(), order.Edges.Token.Decimals)
			amountToBePaid := order.Amount.Add(order.ProtocolFee).Add(order.SenderFee).Add(order.NetworkFee)

			if eventData["type"].(string) != "Transfer" && eventData["to"].(string) != order.Edges.ReceiveAddress.Address && amountPaid != amountToBePaid {
				return nil
			}

			res, err = client.Build().POST("/wallet/gettransactioninfobyid").
				Body().AsJSON(map[string]interface{}{"value": eventData["transaction_id"].(string)}).
				Retry().Set(3, 1*time.Second).
				Send()
			if err != nil {
				logger.Errorf("IndexTRC20Transfer.FetchBlockNumber: %v", err)
				return err
			}

			data, err = utils.ParseJSONResponse(res.RawResponse)
			if err != nil {
				logger.Errorf("IndexTRC20Transfer.ParseJSONResponse: %v", err)
				return err
			}

			logger.Errorf("gettransactioninfobyid: %v", data)

			if data["blockNumber"] != nil {
				transferEvent := &types.TokenTransferEvent{
					BlockNumber: uint64(data["blockNumber"].(float64)),
					TxHash:      eventData["transaction_id"].(string),
					From:        eventData["from"].(string),
					To:          eventData["to"].(string),
					Value:       value.BigInt(),
				}

				go func() {
					_, err := s.UpdateReceiveAddressStatus(ctx, nil, order.Edges.ReceiveAddress, order, transferEvent)
					if err != nil {
						logger.Errorf("IndexTRC20Transfer.UpdateReceiveAddressStatus: %v", err)
					}
				}()
			}
		}
	}

	return nil
}

// IndexOrderCreated indexes deposits to the order contract for an EVM network.
func (s *IndexerService) IndexOrderCreated(ctx context.Context, client types.RPCClient, network *ent.Network, sender string) error {
	var err error

	// Connect to RPC endpoint
	if client == nil {
		retryErr := utils.Retry(3, 1*time.Second, func() error {
			client, err = types.NewEthClient(network.RPCEndpoint)
			return err
		})
		if retryErr != nil {
			logger.Errorf("IndexOrderCreated.NewEthClient: %v", retryErr)
			return retryErr
		}
	}

	// Initialize contract filterer
	filterer, err := contracts.NewGatewayFilterer(common.HexToAddress(network.GatewayContractAddress), client)
	if err != nil {
		logger.Errorf("IndexOrderCreated.NewGatewayFilterer: %v", err)
		return err
	}

	// Fetch current block header
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		logger.Errorf("IndexOrderCreated.HeaderByNumber: %v", err)
	}
	toBlock := header.Number.Uint64()

	// Fetch logs
	var iter *contracts.GatewayOrderCreatedIterator
	retryErr := utils.Retry(3, 1*time.Second, func() error {
		var err error
		if sender != "" {
			iter, err = filterer.FilterOrderCreated(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 5000),
				End:   &toBlock,
			}, []common.Address{common.HexToAddress(sender)}, nil, nil)
		} else {
			iter, err = filterer.FilterOrderCreated(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 5000),
				End:   &toBlock,
			}, nil, nil, nil)
		}
		return err
	})
	if retryErr != nil {
		logger.Errorf("IndexOrderCreated.FilterOrderCreated: %v", retryErr)
		return retryErr
	}

	// Iterate over logs
	for iter.Next() {
		event := &types.OrderCreatedEvent{
			BlockNumber: iter.Event.Raw.BlockNumber,
			TxHash:      iter.Event.Raw.TxHash.Hex(),
			Token:       iter.Event.Token.Hex(),
			Amount:      iter.Event.Amount,
			ProtocolFee: iter.Event.ProtocolFee,
			OrderId:     iter.Event.OrderId,
			Rate:        iter.Event.Rate,
			MessageHash: iter.Event.MessageHash,
		}
		err := s.CreateLockPaymentOrder(ctx, client, network, event)
		if err != nil {
			logger.Errorf("IndexOrderCreated.createOrder: %v", err)
			continue
		}
	}

	return nil
}

// IndexOrderCreatedTron indexes orders created in the Gateway contract for the Tron network.
func (s *IndexerService) IndexOrderCreatedTron(ctx context.Context, order *ent.PaymentOrder) error {
	events, err := s.fetchLatestOrderEvents(
		order.Edges.Token.Edges.Network.RPCEndpoint,
		order.Edges.Token.Edges.Network.Identifier,
		order.TxHash,
	)
	if err != nil {
		return fmt.Errorf("IndexOrderCreatedTron.fetchLatestOrderEvents: %v", err)
	}

	for _, event := range events {
		eventData := event.(map[string]interface{})
		if eventData["event_name"] == "OrderCreated" && eventData["contract_address"] == order.Edges.Token.Edges.Network.GatewayContractAddress {
			client := fastshot.NewClient(order.Edges.Token.Edges.Network.RPCEndpoint).
				Config().SetTimeout(30*time.Second).
				Header().Add("TRON_PRO_API_KEY", orderConf.TronProApiKey)

			res, err := client.Build().POST("/wallet/gettransactioninfobyid").
				Body().AsJSON(map[string]interface{}{"value": order.TxHash}).
				Retry().Set(3, 1*time.Second).
				Send()
			if err != nil {
				logger.Errorf("fetch txn event logs: %v", err)
				return err
			}

			data, err := utils.ParseJSONResponse(res.RawResponse)
			if err != nil {
				logger.Errorf("failed to parse JSON response: %v", err)
				return err
			}

			// Parse event data
			for _, event := range data["log"].([]interface{}) {
				eventData := event.(map[string]interface{})
				if eventData["topics"].([]interface{})[0] == "3bdd0d86e09a22d7ce596118bd3ca5ec73ea47533a465be37621e913ed2bf333" {
					unpackedEventData, err := utils.UnpackEventData(eventData["data"].(string), contracts.GatewayMetaData.ABI, "OrderCreated")
					if err != nil {
						logger.Errorf("IndexOrderCreatedTron.UnpackEventData: %v", err)
						return err
					}

					event := &types.OrderCreatedEvent{
						BlockNumber: uint64(data["blockNumber"].(float64)),
						TxHash:      data["id"].(string),
						Token:       utils.ParseTopicToTronAddress(eventData["topics"].([]interface{})[2].(string)),
						Amount:      utils.ParseTopicToBigInt(eventData["topics"].([]interface{})[3].(string)),
						ProtocolFee: unpackedEventData[0].(*big.Int),
						OrderId:     unpackedEventData[1].([32]byte),
						Rate:        unpackedEventData[2].(*big.Int),
						MessageHash: unpackedEventData[4].(string),
					}

					// Connect to RPC endpoint
					// TODO: rewrite this piece of code to fetch from the Gateway contract on Tron when it has supported institutions added to it
					var ethClient types.RPCClient
					retryErr := utils.Retry(3, 1*time.Second, func() error {
						ethClient, err = types.NewEthClient("https://polygon-mainnet.g.alchemy.com/v2/zfXjaatj2o5xKkqe0iSvnU9JkKZoiS54")
						return err
					})
					if retryErr != nil {
						logger.Errorf("IndexOrderCreatedTron.NewEthClient: %v", retryErr)
						return retryErr
					}

					err = s.CreateLockPaymentOrder(ctx, ethClient, order.Edges.Token.Edges.Network, event)
					if err != nil {
						logger.Errorf("IndexOrderCreatedTron.CreateLockPaymentOrder: %v", err)
					}

					break
				}
			}

			break
		}
	}

	return nil
}

// IndexOrderSettled indexes order settlements for an EVM network.
func (s *IndexerService) IndexOrderSettled(ctx context.Context, client types.RPCClient, network *ent.Network, gatewayId string) error {
	var err error

	// Connect to RPC endpoint
	if client == nil {
		retryErr := utils.Retry(3, 1*time.Second, func() error {
			client, err = types.NewEthClient(network.RPCEndpoint)
			return err
		})
		if retryErr != nil {
			logger.Errorf("IndexOrderSettled.NewEthClient: %v", retryErr)
			return retryErr
		}
	}

	// Initialize contract filterer
	filterer, err := contracts.NewGatewayFilterer(common.HexToAddress(network.GatewayContractAddress), client)
	if err != nil {
		logger.Errorf("IndexOrderSettled.NewGatewayFilterer: %v", err)
		return err
	}

	// Filter logs from the oldest indexed to the latest
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		logger.Errorf("IndexOrderSettled.HeaderByNumber: %v", err)
	}
	toBlock := header.Number.Uint64()

	// Fetch logs
	var iter *contracts.GatewayOrderSettledIterator
	retryErr := utils.Retry(3, 1*time.Second, func() error {
		if gatewayId != "" {
			orderID, err := hex.DecodeString(gatewayId[2:])
			if err != nil {
				logger.Errorf("IndexOrderSettled.DecodeString: %v", err)
				return err
			}

			iter, err = filterer.FilterOrderSettled(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 50000),
				End:   &toBlock,
			}, [][32]byte{utils.StringToByte32(string(orderID))}, nil)

			return err
		} else {
			iter, err = filterer.FilterOrderSettled(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 50000),
				End:   &toBlock,
			}, nil, nil)

			return err
		}
	})
	if retryErr != nil {
		logger.Errorf("IndexOrderSettled.FilterOrderSettled: %v", retryErr)
		return retryErr
	}

	// Iterate over logs
	for iter.Next() {
		settledEvent := &types.OrderSettledEvent{
			BlockNumber:       iter.Event.Raw.BlockNumber,
			TxHash:            iter.Event.Raw.TxHash.Hex(),
			SplitOrderId:      iter.Event.SplitOrderId,
			OrderId:           iter.Event.OrderId,
			LiquidityProvider: iter.Event.LiquidityProvider.Hex(),
			SettlePercent:     iter.Event.SettlePercent,
		}
		err := s.UpdateOrderStatusSettled(ctx, settledEvent)
		if err != nil {
			logger.Errorf("IndexOrderSettled.UpdateOrderStatusSettled: %v", err)
			continue
		}
	}

	return nil
}

// IndexOrderSettledTron indexes orders settled in the Gateway contract for the Tron network.
func (s *IndexerService) IndexOrderSettledTron(ctx context.Context, order *ent.LockPaymentOrder) error {
	events, err := s.fetchLatestOrderEvents(
		order.Edges.Token.Edges.Network.RPCEndpoint,
		order.Edges.Token.Edges.Network.Identifier,
		order.TxHash,
	)
	if err != nil {
		return fmt.Errorf("IndexOrderSettledTron.fetchLatestOrderEvents: %v", err)
	}

	for _, event := range events {
		eventData := event.(map[string]interface{})
		if eventData["event_name"] == "OrderSettled" && eventData["contract_address"] == order.Edges.Token.Edges.Network.GatewayContractAddress {
			client := fastshot.NewClient(order.Edges.Token.Edges.Network.RPCEndpoint).
				Config().SetTimeout(30*time.Second).
				Header().Add("TRON_PRO_API_KEY", orderConf.TronProApiKey)

			res, err := client.Build().POST("/wallet/gettransactioninfobyid").
				Body().AsJSON(map[string]interface{}{"value": order.TxHash}).
				Retry().Set(3, 1*time.Second).
				Send()
			if err != nil {
				logger.Errorf("fetch txn event logs: %v", err)
				return err
			}

			data, err := utils.ParseJSONResponse(res.RawResponse)
			if err != nil {
				logger.Errorf("failed to parse JSON response: %v", err)
				return err
			}

			// Parse event data
			for _, event := range data["log"].([]interface{}) {
				eventData := event.(map[string]interface{})
				if eventData["topics"].([]interface{})[0] == "98ece21e01a01cbe1d1c0dad3b053c8fbd368f99be78be958fcf1d1d13fd249a" {
					unpackedEventData, err := utils.UnpackEventData(eventData["data"].(string), contracts.GatewayMetaData.ABI, "OrderSettled")
					if err != nil {
						logger.Errorf("IndexOrderSettledTron.UnpackEventData: %v", err)
						return err
					}

					event := &types.OrderSettledEvent{
						BlockNumber:       uint64(data["blockNumber"].(float64)),
						TxHash:            data["id"].(string),
						SplitOrderId:      unpackedEventData[0].([32]byte),
						OrderId:           utils.ParseTopicToByte32(eventData["topics"].([]interface{})[1].(string)),
						LiquidityProvider: eventData["topics"].([]interface{})[2].(string),
						SettlePercent:     unpackedEventData[1].(*big.Int),
					}

					err = s.UpdateOrderStatusSettled(ctx, event)
					if err != nil {
						logger.Errorf("IndexOrderSettledTron.UpdateOrderStatusSettled: %v", err)
					}

					break
				}
			}

			break
		}
	}

	return nil
}

// IndexOrderRefunded indexes order refunds for an EVM network.
func (s *IndexerService) IndexOrderRefunded(ctx context.Context, client types.RPCClient, network *ent.Network, gatewayId string) error {
	var err error

	// Connect to RPC endpoint
	if client == nil {
		retryErr := utils.Retry(3, 1*time.Second, func() error {
			client, err = types.NewEthClient(network.RPCEndpoint)
			return err
		})
		if retryErr != nil {
			logger.Errorf("IndexOrderRefunded.NewEthClient: %v", err)
			return retryErr
		}
	}

	// Initialize contract filterer
	filterer, err := contracts.NewGatewayFilterer(common.HexToAddress(network.GatewayContractAddress), client)
	if err != nil {
		logger.Errorf("IndexOrderRefunded.NewGatewayFilterer: %v", err)
		return err
	}

	// Filter logs from the oldest indexed to the latest
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		logger.Errorf("IndexOrderRefunded.HeaderByNumber: %v", err)
	}
	toBlock := header.Number.Uint64()

	// Fetch logs
	var iter *contracts.GatewayOrderRefundedIterator
	retryErr := utils.Retry(3, 1*time.Second, func() error {
		if gatewayId != "" {
			orderID, err := hex.DecodeString(gatewayId[2:])
			if err != nil {
				logger.Errorf("IndexOrderRefunded.DecodeString: %v", err)
				return err
			}

			iter, err = filterer.FilterOrderRefunded(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 50000),
				End:   &toBlock,
			}, [][32]byte{utils.StringToByte32(string(orderID))})
			return err
		} else {
			iter, err = filterer.FilterOrderRefunded(&bind.FilterOpts{
				Start: uint64(int64(toBlock) - 50000),
				End:   &toBlock,
			}, nil)
			return err
		}
	})
	if retryErr != nil {
		logger.Errorf("IndexOrderRefunded.FilterOrderRefunded: %v", retryErr)
		return retryErr
	}

	// Iterate over logs
	for iter.Next() {
		refundedEvent := &types.OrderRefundedEvent{
			BlockNumber: iter.Event.Raw.BlockNumber,
			TxHash:      iter.Event.Raw.TxHash.Hex(),
			Fee:         iter.Event.Fee,
			OrderId:     iter.Event.OrderId,
		}
		err := s.UpdateOrderStatusRefunded(ctx, refundedEvent)
		if err != nil {
			logger.Errorf("IndexOrderRefunded.UpdateOrderStatusRefunded: %v", err)
			continue
		}
	}

	return nil
}

// IndexOrderRefundedTron indexes orders settled in the Gateway contract for the Tron network.
func (s *IndexerService) IndexOrderRefundedTron(ctx context.Context, order *ent.LockPaymentOrder) error {
	events, err := s.fetchLatestOrderEvents(
		order.Edges.Token.Edges.Network.RPCEndpoint,
		order.Edges.Token.Edges.Network.Identifier,
		order.TxHash,
	)
	if err != nil {
		return fmt.Errorf("IndexOrderRefundedTron.fetchLatestOrderEvents: %v", err)
	}

	for _, event := range events {
		eventData := event.(map[string]interface{})
		if eventData["event_name"] == "OrderRefunded" && eventData["contract_address"] == order.Edges.Token.Edges.Network.GatewayContractAddress {
			client := fastshot.NewClient(order.Edges.Token.Edges.Network.RPCEndpoint).
				Config().SetTimeout(30*time.Second).
				Header().Add("TRON_PRO_API_KEY", orderConf.TronProApiKey)

			res, err := client.Build().POST("/wallet/gettransactioninfobyid").
				Body().AsJSON(map[string]interface{}{"value": order.TxHash}).
				Retry().Set(3, 1*time.Second).
				Send()
			if err != nil {
				logger.Errorf("fetch txn event logs: %v", err)
				return err
			}

			data, err := utils.ParseJSONResponse(res.RawResponse)
			if err != nil {
				logger.Errorf("failed to parse JSON response: %v", err)
				return err
			}

			// Parse event data
			for _, event := range data["log"].([]interface{}) {
				eventData := event.(map[string]interface{})
				if eventData["topics"].([]interface{})[0] == "0736fe428e1747ca8d387c2e6fa1a31a0cde62d3a167c40a46ade59a3cdc828e" {
					unpackedEventData, err := utils.UnpackEventData(eventData["data"].(string), contracts.GatewayMetaData.ABI, "OrderRefunded")
					if err != nil {
						logger.Errorf("IndexOrderRefundedTron.UnpackEventData: %v", err)
						return err
					}

					event := &types.OrderRefundedEvent{
						BlockNumber: uint64(data["blockNumber"].(float64)),
						TxHash:      data["id"].(string),
						OrderId:     eventData["topics"].([]interface{})[1].([32]byte),
						Fee:         unpackedEventData[0].(*big.Int),
					}

					err = s.UpdateOrderStatusRefunded(ctx, event)
					if err != nil {
						logger.Errorf("IndexOrderRefundedTron.UpdateOrderStatusRefunded: %v", err)
					}

					break
				}
			}

			break
		}
	}

	return nil
}

// HandleReceiveAddressValidity checks the validity of a receive address
func (s *IndexerService) HandleReceiveAddressValidity(ctx context.Context, client types.RPCClient, receiveAddress *ent.ReceiveAddress, paymentOrder *ent.PaymentOrder) error {
	if receiveAddress.ValidUntil.IsZero() {
		return nil
	}

	if receiveAddress.Status != receiveaddress.StatusUsed {
		amountNotPaidInFull := receiveAddress.Status == receiveaddress.StatusPartial || receiveAddress.Status == receiveaddress.StatusUnused
		validUntilIsFarGone := receiveAddress.ValidUntil.Before(time.Now().Add(-(5 * time.Minute)))
		isExpired := receiveAddress.ValidUntil.Before(time.Now())

		if validUntilIsFarGone {
			_, err := receiveAddress.
				Update().
				SetValidUntil(time.Now().Add(orderConf.ReceiveAddressValidity)).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("HandleReceiveAddressValidity.db: %v", err)
			}
		} else if isExpired && amountNotPaidInFull {
			// Receive address hasn't received full payment after validity period, mark status as expired
			_, err := receiveAddress.
				Update().
				SetStatus(receiveaddress.StatusExpired).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("HandleReceiveAddressValidity.db: %v", err)
			}

			// Expire payment order
			_, err = paymentOrder.
				Update().
				SetStatus(paymentorder.StatusExpired).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("HandleReceiveAddressValidity.db: %v", err)
			}

			// Revert amount to the from address
			err = s.order.RevertOrder(ctx, client, paymentOrder)
			if err != nil {
				return fmt.Errorf("HandleReceiveAddressValidity.RevertOrder: %v", err)
			}
		}
	} else {
		// Revert excess amount to the from address
		err := s.order.RevertOrder(ctx, client, paymentOrder)
		if err != nil {
			return fmt.Errorf("HandleReceiveAddressValidity.RevertOrder: %v", err)
		}
	}

	return nil
}

// CreateLockPaymentOrder saves a lock payment order in the database
func (s *IndexerService) CreateLockPaymentOrder(ctx context.Context, client types.RPCClient, network *ent.Network, event *types.OrderCreatedEvent) error {
	gatewayId := fmt.Sprintf("0x%v", hex.EncodeToString(event.OrderId[:]))

	// Check for existing address with txHash
	orderCount, err := db.Client.LockPaymentOrder.
		Query().
		Where(
			lockpaymentorder.Or(
				lockpaymentorder.TxHashEQ(event.TxHash),
				lockpaymentorder.GatewayIDEQ(gatewayId),
			),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("CreateLockPaymentOrder.db: %v", err)
	}

	if orderCount > 0 {
		// This transfer has already been indexed
		return nil
	}

	// Update payment order with the gateway ID
	paymentOrderExists := true
	paymentOrder, err := db.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.TxHashEQ(event.TxHash),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Payment order does not exist, no need to update
			paymentOrderExists = false
		} else {
			return fmt.Errorf("CreateLockPaymentOrder.db: %v", err)
		}
	}

	if paymentOrderExists {
		_, err = paymentOrder.
			Update().
			SetBlockNumber(int64(event.BlockNumber)).
			SetGatewayID(gatewayId).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("CreateLockPaymentOrder.db: %v", err)
		}
	}

	// Get token from db
	token, err := db.Client.Token.
		Query().
		Where(
			token.ContractAddressEQ(event.Token),
			token.HasNetworkWith(
				networkent.IDEQ(network.ID),
			),
		).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch token: %w", err)
	}

	// Get order recipient from message hash
	recipient, err := s.getOrderRecipientFromMessageHash(event.MessageHash)
	if err != nil {
		return fmt.Errorf("failed to decrypt message hash: %w", err)
	}

	// Get provision bucket
	amountInDecimals := utils.FromSubunit(event.Amount, token.Decimals)
	institution, err := s.getInstitutionByCode(ctx, recipient.Institution)
	if err != nil {
		return fmt.Errorf("failed to fetch institution: %w", err)
	}

	currency, err := db.Client.FiatCurrency.
		Query().
		Where(
			fiatcurrency.IsEnabledEQ(true),
			fiatcurrency.CodeEQ(institution.Edges.FiatCurrency.Code),
		).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch fiat currency: %w", err)
	}

	rate := decimal.NewFromBigInt(event.Rate, 0)

	provisionBucket, err := s.getProvisionBucket(ctx, amountInDecimals.Mul(rate).RoundBank(0), currency)
	if err != nil {
		logger.Errorf("failed to fetch provision bucket: %v", err)
	}

	// Create lock payment order fields
	lockPaymentOrder := types.LockPaymentOrderFields{
		Token:             token,
		Network:           network,
		GatewayID:         gatewayId,
		Amount:            amountInDecimals,
		Rate:              rate,
		BlockNumber:       int64(event.BlockNumber),
		TxHash:            event.TxHash,
		Institution:       recipient.Institution,
		AccountIdentifier: recipient.AccountIdentifier,
		AccountName:       recipient.AccountName,
		ProviderID:        recipient.ProviderID,
		Memo:              recipient.Memo,
		ProvisionBucket:   provisionBucket,
	}

	// Check if order is private
	isPrivate := false
	maxOrderAmount := decimal.NewFromInt(0)
	if lockPaymentOrder.ProviderID != "" {
		providerProfile, err := db.Client.ProviderProfile.
			Query().
			Where(
				providerprofile.IDEQ(recipient.ProviderID),
			).
			WithOrderTokens().
			Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch provider: %w", err)
		}
		maxOrderAmount = providerProfile.Edges.OrderTokens[0].MaxOrderAmount

		if providerProfile.VisibilityMode == providerprofile.VisibilityModePrivate {
			isPrivate = true
		}
	}

	if provisionBucket == nil && !isPrivate {
		if institution.Edges.FiatCurrency == nil || !institution.Edges.FiatCurrency.IsEnabled {
			return fmt.Errorf("failed to fetch an active fiat currency")
		}
		currency := institution.Edges.FiatCurrency

		// Split lock payment order into multiple orders
		err = s.splitLockPaymentOrder(
			ctx, client, lockPaymentOrder, currency,
		)
		if err != nil {
			return fmt.Errorf("%s - failed to split lock payment order: %w", lockPaymentOrder.GatewayID, err)
		}
	} else {
		// Create LockPaymentOrder and recipient in a transaction
		tx, err := db.Client.Tx(ctx)
		if err != nil {
			return fmt.Errorf("%s failed to initiate db transaction %w", lockPaymentOrder.GatewayID, err)
		}

		transactionLog, err := tx.TransactionLog.
			Create().
			SetStatus(transactionlog.StatusOrderCreated).
			SetTxHash(lockPaymentOrder.TxHash).
			SetNetwork(network.Identifier).
			SetGatewayID(lockPaymentOrder.GatewayID).
			SetMetadata(
				map[string]interface{}{
					"Token":           token,
					"GatewayID":       gatewayId,
					"Amount":          amountInDecimals,
					"Rate":            rate,
					"Memo":            recipient.Memo,
					"ProvisionBucket": provisionBucket,
					"ProviderID":      lockPaymentOrder.ProviderID,
				}).
			Save(ctx)

		if err != nil {
			return fmt.Errorf("%s - failed to create transaction Log : %w", lockPaymentOrder.GatewayID, err)
		}
		// Create lock payment order in db
		orderBuilder := tx.LockPaymentOrder.
			Create().
			SetToken(lockPaymentOrder.Token).
			SetGatewayID(lockPaymentOrder.GatewayID).
			SetAmount(lockPaymentOrder.Amount).
			SetRate(lockPaymentOrder.Rate).
			SetOrderPercent(decimal.NewFromInt(100)).
			SetBlockNumber(lockPaymentOrder.BlockNumber).
			SetTxHash(lockPaymentOrder.TxHash).
			SetInstitution(lockPaymentOrder.Institution).
			SetAccountIdentifier(lockPaymentOrder.AccountIdentifier).
			SetAccountName(lockPaymentOrder.AccountName).
			SetMemo(lockPaymentOrder.Memo).
			SetProvisionBucket(lockPaymentOrder.ProvisionBucket).
			AddTransactions(transactionLog)

		if lockPaymentOrder.ProviderID != "" {
			orderBuilder = orderBuilder.SetProviderID(lockPaymentOrder.ProviderID)
		}

		orderCreated, err := orderBuilder.Save(ctx)
		if err != nil {
			return fmt.Errorf("%s - failed to create lock payment order: %w", lockPaymentOrder.GatewayID, err)
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("%s - failed to create lock payment order: %w", lockPaymentOrder.GatewayID, err)
		}

		// Check AML compliance
		if serverConf.Environment == "production" && !strings.HasPrefix(network.Identifier, "tron") {
			ok, err := s.checkAMLCompliance(network.RPCEndpoint, event.TxHash)
			if err != nil {
				logger.Errorf("checkAMLCompliance: %v", err)
			}

			if !ok && err == nil {
				err := s.order.RefundOrder(ctx, client, lockPaymentOrder.GatewayID)
				if err != nil {
					return fmt.Errorf("checkAMLCompliance.RefundOrder: %w", err)
				}
				return nil
			}
		}

		// Assign the lock payment order to a provider
		if isPrivate && lockPaymentOrder.Amount.GreaterThan(maxOrderAmount) {
			err := s.order.RefundOrder(ctx, client, lockPaymentOrder.GatewayID)
			if err != nil {
				return fmt.Errorf("failed to refund order: %w", err)
			}
			return nil
		} else {
			lockPaymentOrder.ID = orderCreated.ID
			_ = s.priorityQueue.AssignLockPaymentOrder(ctx, lockPaymentOrder)
		}
	}

	return nil
}

// UpdateOrderStatusRefunded updates the status of a payment order to refunded
func (s *IndexerService) UpdateOrderStatusRefunded(ctx context.Context, log *types.OrderRefundedEvent) error {
	gatewayId := fmt.Sprintf("0x%v", hex.EncodeToString(log.OrderId[:]))

	// Fetch payment order
	paymentOrderExists := true
	paymentOrder, err := db.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.GatewayIDEQ(gatewayId),
		).
		WithSenderProfile().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Payment order does not exist, no need to update
			paymentOrderExists = false
		} else {
			return fmt.Errorf("UpdateOrderStatusRefunded.fetchOrder: %v", err)
		}
	}

	tx, err := db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusRefunded.dbtransaction %v", err)
	}

	// Attempt to update an existing log
	var transactionLog *ent.TransactionLog
	updatedRows, err := tx.TransactionLog.
		Update().
		Where(
			transactionlog.StatusEQ(transactionlog.StatusOrderRefunded),
			transactionlog.GatewayIDEQ(gatewayId),
		).
		SetTxHash(log.TxHash).
		SetMetadata(
			map[string]interface{}{
				"OrderId":         log.OrderId,
				"GatewayID":       gatewayId,
				"TransactionData": log,
			}).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusRefunded.update: %v", err)
	}

	// If no rows were updated, create a new log
	if updatedRows == 0 {
		transactionLog, err = tx.TransactionLog.
			Create().
			SetStatus(transactionlog.StatusOrderRefunded).
			SetTxHash(log.TxHash).
			SetGatewayID(gatewayId).
			SetMetadata(
				map[string]interface{}{
					"OrderId":         log.OrderId,
					"GatewayID":       gatewayId,
					"TransactionData": log,
				}).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusRefunded.create: %v", err)
		}
	}

	// Aggregator side status update
	lockPaymentOrderUpdate := tx.LockPaymentOrder.
		Update().
		Where(
			lockpaymentorder.GatewayIDEQ(gatewayId),
		).
		SetBlockNumber(int64(log.BlockNumber)).
		SetTxHash(log.TxHash).
		SetStatus(lockpaymentorder.StatusRefunded)

	if transactionLog != nil {
		lockPaymentOrderUpdate = lockPaymentOrderUpdate.AddTransactions(transactionLog)
	}

	_, err = lockPaymentOrderUpdate.Save(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusRefunded.aggregator: %v", err)
	}

	// Sender side status update
	if paymentOrderExists {
		_, err = tx.PaymentOrder.
			Update().
			Where(
				paymentorder.GatewayIDEQ(gatewayId),
			).
			SetTxHash(log.TxHash).
			SetStatus(paymentorder.StatusRefunded).
			AddTransactions(transactionLog).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusRefunded.sender: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UpdateOrderStatusRefunded.commit %v", err)
	}

	if paymentOrderExists {
		paymentOrder.Status = paymentorder.StatusRefunded
		paymentOrder.TxHash = log.TxHash

		// Send webhook notification to sender
		err = utils.SendPaymentOrderWebhook(ctx, paymentOrder)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusRefunded.webhook: %v", err)
		}
	}

	return nil
}

// UpdateOrderStatusSettled updates the status of a payment order to settled
func (s *IndexerService) UpdateOrderStatusSettled(ctx context.Context, event *types.OrderSettledEvent) error {
	gatewayId := fmt.Sprintf("0x%v", hex.EncodeToString(event.OrderId[:]))

	// Fetch payment order
	paymentOrderExists := true
	paymentOrder, err := db.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.GatewayIDEQ(gatewayId),
		).
		WithSenderProfile().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Payment order does not exist, no need to update
			paymentOrderExists = false
		} else {
			return fmt.Errorf("UpdateOrderStatusSettled.fetchOrder: %v", err)
		}
	}

	tx, err := db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusSettled.db: %v", err)
	}

	// Attempt to update an existing log
	var transactionLog *ent.TransactionLog
	updatedRows, err := tx.TransactionLog.
		Update().
		Where(
			transactionlog.StatusEQ(transactionlog.StatusOrderSettled),
			transactionlog.GatewayIDEQ(gatewayId),
		).
		SetTxHash(event.TxHash).
		SetMetadata(map[string]interface{}{
			"GatewayID":       gatewayId,
			"TransactionData": event,
		}).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusSettled.update: %v", err)
	}

	// If no rows were updated, create a new log
	if updatedRows == 0 {
		transactionLog, err = tx.TransactionLog.
			Create().
			SetStatus(transactionlog.StatusOrderSettled).
			SetTxHash(event.TxHash).
			SetGatewayID(gatewayId).
			SetMetadata(map[string]interface{}{
				"GatewayID":       gatewayId,
				"TransactionData": event,
			}).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusSettled.create: %v", err)
		}
	}

	// Aggregator side status update
	splitOrderId, _ := uuid.Parse(utils.Byte32ToString(event.SplitOrderId))
	lockPaymentOrderUpdate := tx.LockPaymentOrder.
		Update().
		Where(
			lockpaymentorder.IDEQ(splitOrderId),
		).
		SetBlockNumber(int64(event.BlockNumber)).
		SetTxHash(event.TxHash).
		SetStatus(lockpaymentorder.StatusSettled)

	if transactionLog != nil {
		lockPaymentOrderUpdate = lockPaymentOrderUpdate.AddTransactions(transactionLog)
	}

	_, err = lockPaymentOrderUpdate.Save(ctx)
	if err != nil {
		return fmt.Errorf("UpdateOrderStatusSettled.aggregator: %v", err)
	}

	// Sender side status update
	if paymentOrderExists {
		paymentOrderUpdate := tx.PaymentOrder.
			Update().
			Where(
				paymentorder.GatewayIDEQ(gatewayId),
			)

		// Convert settled percent to BPS
		settledPercent := decimal.NewFromBigInt(event.SettlePercent, 0).Div(decimal.NewFromInt(1000))

		// If settled percent is 100%, mark order as settled
		if settledPercent.Equal(decimal.NewFromInt(100)) {
			paymentOrderUpdate = paymentOrderUpdate.SetStatus(paymentorder.StatusSettled)
		}

		_, err = paymentOrderUpdate.
			SetBlockNumber(int64(event.BlockNumber)).
			SetTxHash(event.TxHash).
			SetPercentSettled(settledPercent).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusSettled.sender: %v", err)
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("UpdateOrderStatusSettled.sender %v", err)
		}

		// Send webhook notifcation to sender
		paymentOrder.BlockNumber = int64(event.BlockNumber)
		paymentOrder.TxHash = event.TxHash
		paymentOrder.PercentSettled = settledPercent

		err = utils.SendPaymentOrderWebhook(ctx, paymentOrder)
		if err != nil {
			return fmt.Errorf("UpdateOrderStatusSettled.webhook: %v", err)
		}
	}

	return nil
}

// getOrderRecipientFromMessageHash decrypts the message hash and returns the order recipient
func (s *IndexerService) getOrderRecipientFromMessageHash(messageHash string) (*types.PaymentOrderRecipient, error) {
	messageCipher, err := base64.StdEncoding.DecodeString(messageHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message hash: %w", err)
	}

	// Decrypt with the private key of the aggregator
	message, err := cryptoUtils.PublicKeyDecryptJSON(messageCipher, config.CryptoConfig().AggregatorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message hash: %w", err)
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	var recipient *types.PaymentOrderRecipient
	if err := json.Unmarshal(messageBytes, &recipient); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return recipient, nil
}

// UpdateReceiveAddressStatus updates the status of a receive address. if `done` is true, the indexing process is complete for the given receive address
func (s *IndexerService) UpdateReceiveAddressStatus(
	ctx context.Context, client types.RPCClient, receiveAddress *ent.ReceiveAddress, paymentOrder *ent.PaymentOrder, event *types.TokenTransferEvent,
) (done bool, err error) {
	if event.To == receiveAddress.Address {
		// Check for existing address with txHash
		count, err := db.Client.ReceiveAddress.
			Query().
			Where(receiveaddress.TxHashEQ(event.TxHash)).
			Count(ctx)
		if err != nil {
			return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
		}

		if count > 0 && receiveAddress.Status != receiveaddress.StatusUnused {
			// This transfer has already been indexed
			return false, nil
		}

		// Check for existing payment order with txHash
		if paymentOrder.TxHash == event.TxHash {
			// This transfer has already been indexed
			return false, nil
		}

		// This is a transfer to the receive address to create an order on-chain
		// Compare the transferred value with the expected order amount + fees
		fees := paymentOrder.NetworkFee.Add(paymentOrder.SenderFee).Add(paymentOrder.ProtocolFee)
		orderAmountWithFees := paymentOrder.Amount.Add(fees).Round(int32(paymentOrder.Edges.Token.Decimals))
		orderAmountWithFeesInSubunit := utils.ToSubunit(orderAmountWithFees, paymentOrder.Edges.Token.Decimals)
		comparisonResult := event.Value.Cmp(orderAmountWithFeesInSubunit)

		paymentOrderUpdate := db.Client.PaymentOrder.Update().Where(paymentorder.IDEQ(paymentOrder.ID))
		if paymentOrder.ReturnAddress == "" {
			paymentOrderUpdate = paymentOrderUpdate.SetReturnAddress(event.From)
		}

		orderRecipient := paymentOrder.Edges.Recipient
		if strings.HasPrefix(orderRecipient.Memo, "P#P") && orderRecipient.ProviderID != "" && comparisonResult != 0 {
			// This is a P2P order created from the provider dashboard. No reverts are allowed
			// Hence, the order amount will be updated to whatever amount was sent to the receive address
			orderAmount := utils.FromSubunit(event.Value, paymentOrder.Edges.Token.Decimals)
			paymentOrderUpdate.SetAmount(orderAmount.Round(int32(2)))

			// Update the rate with the current rate if order is older than 30 mins
			if paymentOrder.CreatedAt.Before(time.Now().Add(-30 * time.Minute)) {
				providerProfile, err := db.Client.ProviderProfile.
					Query().
					Where(
						providerprofile.HasUserWith(
							user.HasSenderProfileWith(
								senderprofile.HasPaymentOrdersWith(
									paymentorder.IDEQ(paymentOrder.ID),
								),
							),
						),
					).
					Only(ctx)
				if err != nil {
					return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
				}

				rate, err := s.priorityQueue.GetProviderRate(ctx, providerProfile)
				if err != nil {
					return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
				}
				paymentOrderUpdate.SetRate(rate)
			}
			comparisonResult = 0
		}

		transactionLog, err := db.Client.TransactionLog.
			Create().
			SetStatus(transactionlog.StatusCryptoDeposited).
			SetGatewayID(paymentOrder.GatewayID).
			SetTxHash(event.TxHash).
			SetNetwork(paymentOrder.Edges.Token.Edges.Network.Identifier).
			SetMetadata(map[string]interface{}{
				"GatewayID":       paymentOrder.GatewayID,
				"transactionData": event,
			}).
			Save(ctx)

		if err != nil {
			return true, fmt.Errorf("UpdateReceiveAddressStatus.transactionlog: %v", err)
		}

		_, err = paymentOrderUpdate.
			SetFromAddress(event.From).
			SetTxHash(event.TxHash).
			AddAmountPaid(utils.FromSubunit(event.Value, paymentOrder.Edges.Token.Decimals)).
			AddTransactions(transactionLog).
			Save(ctx)
		if err != nil {
			return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
		}

		if comparisonResult == 0 {
			// Transfer value equals order amount with fees
			_, err = receiveAddress.
				Update().
				SetStatus(receiveaddress.StatusUsed).
				SetLastUsed(time.Now()).
				SetTxHash(event.TxHash).
				SetLastIndexedBlock(int64(event.BlockNumber)).
				Save(ctx)
			if err != nil {
				return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
			}

			err = s.order.CreateOrder(ctx, client, paymentOrder.ID)
			if err != nil {
				return true, fmt.Errorf("UpdateReceiveAddressStatus.CreateOrder: %v", err)
			}

			return true, nil

		} else if comparisonResult < 0 {
			// Transfer value is less than order amount with fees
			// and
			// If amount paid meets or exceeds the order amount with fees, mark receive address as used
			if paymentOrder.AmountPaid.GreaterThanOrEqual(orderAmountWithFees) {
				_, err = receiveAddress.
					Update().
					SetStatus(receiveaddress.StatusUsed).
					SetLastUsed(time.Now()).
					SetTxHash(event.TxHash).
					SetLastIndexedBlock(int64(event.BlockNumber)).
					Save(ctx)
				if err != nil {
					return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
				}
			} else {
				_, err = receiveAddress.
					Update().
					SetStatus(receiveaddress.StatusPartial).
					SetTxHash(event.TxHash).
					SetLastIndexedBlock(int64(event.BlockNumber)).
					Save(ctx)
				if err != nil {
					return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
				}
			}

		} else if comparisonResult > 0 {
			// Transfer value is greater than order amount with fees
			_, err = receiveAddress.
				Update().
				SetStatus(receiveaddress.StatusUsed).
				SetLastUsed(time.Now()).
				SetLastIndexedBlock(int64(event.BlockNumber)).
				Save(ctx)
			if err != nil {
				return true, fmt.Errorf("UpdateReceiveAddressStatus.db: %v", err)
			}
		}

		err = s.HandleReceiveAddressValidity(ctx, client, receiveAddress, paymentOrder)
		if err != nil {
			return true, fmt.Errorf("UpdateReceiveAddressStatus.HandleReceiveAddressValidity: %v", err)
		}
	}

	return false, nil
}

// fetchLatestOrderEvents fetches the latest events of the given order from the Tron network.
func (s *IndexerService) fetchLatestOrderEvents(rpcEndpoint, network, txHash string) ([]interface{}, error) {
	var err error

	if !strings.HasPrefix(network, "tron") {
		return nil, fmt.Errorf("invalid network identifier: %s", network)
	}

	client := fastshot.NewClient(rpcEndpoint).
		Config().SetTimeout(30*time.Second).
		Header().Add("TRON_PRO_API_KEY", orderConf.TronProApiKey)

	// TODO: should we include '?only_confirmed=true' in the URL?
	res, err := client.Build().
		GET(fmt.Sprintf("/v1/transactions/%s/events", txHash)).
		Retry().Set(3, 1*time.Second).
		Send()
	if err != nil {
		logger.Errorf("fetch txn event logs: %v", err)
		return nil, err
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		logger.Errorf("failed to parse JSON response: %v", err)
		return nil, err
	}

	if data["success"].(bool) {
		return data["data"].([]interface{}), nil
	}

	return nil, fmt.Errorf("failed to fetch txn event logs: %v", data["error"])
}

// getProvisionBucket returns the provision bucket for a lock payment order
func (s *IndexerService) getProvisionBucket(ctx context.Context, amount decimal.Decimal, currency *ent.FiatCurrency) (*ent.ProvisionBucket, error) {
	provisionBucket, err := db.Client.ProvisionBucket.
		Query().
		Where(
			provisionbucket.MaxAmountGTE(amount),
			provisionbucket.MinAmountLTE(amount),
			provisionbucket.HasCurrencyWith(
				fiatcurrency.IDEQ(currency.ID),
			),
		).
		WithCurrency().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provision bucket: %w", err)
	}

	return provisionBucket, nil
}

// getInstitutionByCode returns the institution for a given institution code
func (s *IndexerService) getInstitutionByCode(ctx context.Context, institutionCode string) (*ent.Institution, error) {
	institution, err := db.Client.Institution.
		Query().
		Where(institution.CodeEQ(institutionCode)).
		WithFiatCurrency().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return institution, nil
}

// splitLockPaymentOrder splits a lock payment order into multiple orders
func (s *IndexerService) splitLockPaymentOrder(ctx context.Context, client types.RPCClient, lockPaymentOrder types.LockPaymentOrderFields, currency *ent.FiatCurrency) error {
	buckets, err := db.Client.ProvisionBucket.
		Query().
		Where(provisionbucket.HasCurrencyWith(
			fiatcurrency.IDEQ(currency.ID),
		)).
		WithProviderProfiles().
		Order(ent.Desc(provisionbucket.FieldMaxAmount)).
		All(ctx)
	if err != nil {
		logger.Errorf("failed to fetch provision buckets: %v", err)
		return err
	}

	amountToSplit := lockPaymentOrder.Amount.Mul(lockPaymentOrder.Rate).RoundBank(0) // e.g 100,000

	var isRefunded bool

	for _, bucket := range buckets {
		// Get the number of providers in the bucket
		bucketSize := int64(len(bucket.Edges.ProviderProfiles))

		// Get the number of allocations to make in the bucket
		numberOfAllocations := amountToSplit.Div(bucket.MaxAmount).IntPart() // e.g 100000 / 10000 = 10

		var trips int64

		if bucketSize >= numberOfAllocations {
			trips = numberOfAllocations // e.g 10
		} else {
			trips = bucketSize // e.g 2
		}

		// Create a slice to hold the LockPaymentOrder entities for this bucket
		lockOrders := make([]*ent.LockPaymentOrderCreate, 0, trips)

		tx, err := db.Client.Tx(ctx)
		if err != nil {
			return err
		}

		for i := int64(0); i < trips; i++ {
			ratio := bucket.MaxAmount.Div(amountToSplit)
			orderPercent := ratio.Mul(decimal.NewFromInt(100))
			lockOrder := tx.LockPaymentOrder.
				Create().
				SetToken(lockPaymentOrder.Token).
				SetGatewayID(lockPaymentOrder.GatewayID).
				SetAmount(bucket.MaxAmount.Div(lockPaymentOrder.Rate)).
				SetRate(lockPaymentOrder.Rate).
				SetOrderPercent(orderPercent).
				SetBlockNumber(lockPaymentOrder.BlockNumber).
				SetTxHash(lockPaymentOrder.TxHash).
				SetInstitution(lockPaymentOrder.Institution).
				SetAccountIdentifier(lockPaymentOrder.AccountIdentifier).
				SetAccountName(lockPaymentOrder.AccountName).
				SetProviderID(lockPaymentOrder.ProviderID).
				SetProvisionBucket(bucket)
			lockOrders = append(lockOrders, lockOrder)
		}

		// Batch insert all LockPaymentOrder entities for this bucket in a single transaction
		ordersCreated, err := tx.LockPaymentOrder.
			CreateBulk(lockOrders...).
			Save(ctx)
		if err != nil {
			logger.Errorf("failed to create lock payment orders in bulk: %v", err)
			_ = tx.Rollback()
			return err
		}

		// Commit the transaction if everything succeeded
		if err := tx.Commit(); err != nil {
			logger.Errorf("failed to split lock payment order: %v", err)
			return err
		}

		// Check AML compliance
		if serverConf.Environment == "production" && !strings.HasPrefix(lockPaymentOrder.Network.Identifier, "tron") {
			ok, err := s.checkAMLCompliance(lockPaymentOrder.Network.RPCEndpoint, lockPaymentOrder.TxHash)
			if err != nil {
				logger.Errorf("splitLockPaymentOrder.checkAMLCompliance: %v", err)
			}

			if !ok && err == nil {
				isRefunded = true
				err := s.order.RefundOrder(ctx, client, lockPaymentOrder.GatewayID)
				if err != nil {
					logger.Errorf("splitLockPaymentOrder.checkAMLCompliance.RefundOrder: %v", err)
				}
				break
			}
		}

		// Assign the lock payment orders to providers
		for _, order := range ordersCreated {
			lockPaymentOrder.ID = order.ID
			_ = s.priorityQueue.AssignLockPaymentOrder(ctx, lockPaymentOrder)
		}

		amountToSplit = amountToSplit.Sub(bucket.MaxAmount)
	}

	largestBucket := buckets[0]

	if amountToSplit.LessThan(largestBucket.MaxAmount) {
		bucket, err := s.getProvisionBucket(ctx, amountToSplit, currency)
		if err != nil {
			return err
		}

		orderCreatedUpdate := db.Client.LockPaymentOrder.
			Create().
			SetToken(lockPaymentOrder.Token).
			SetGatewayID(lockPaymentOrder.GatewayID).
			SetAmount(amountToSplit.Div(lockPaymentOrder.Rate)).
			SetRate(lockPaymentOrder.Rate).
			SetBlockNumber(lockPaymentOrder.BlockNumber).
			SetTxHash(lockPaymentOrder.TxHash).
			SetInstitution(lockPaymentOrder.Institution).
			SetAccountIdentifier(lockPaymentOrder.AccountIdentifier).
			SetAccountName(lockPaymentOrder.AccountName).
			SetProviderID(lockPaymentOrder.ProviderID).
			SetProvisionBucket(bucket)

		if isRefunded {
			orderCreatedUpdate = orderCreatedUpdate.SetStatus(lockpaymentorder.StatusRefunded)
		}

		orderCreated, err := orderCreatedUpdate.Save(ctx)
		if err != nil {
			logger.Errorf("failed to create lock payment order: %v", err)
			return err
		}

		if !isRefunded {
			// Assign the lock payment order to a provider
			lockPaymentOrder.ID = orderCreated.ID
			_ = s.priorityQueue.AssignLockPaymentOrder(ctx, lockPaymentOrder)
		}
	} else {
		// TODO: figure out how to handle this case, currently it recursively splits the amount
		lockPaymentOrder.Amount = amountToSplit.Div(lockPaymentOrder.Rate)
		err := s.splitLockPaymentOrder(ctx, client, lockPaymentOrder, currency)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkAMLCompliance checks if a transaction is compliant with AML rules
func (s *IndexerService) checkAMLCompliance(rpcUrl string, txHash string) (bool, error) {
	type Transaction struct {
		Kind int         `json:"__kind"`
		Data interface{} `json:"data"`
	}

	type Response struct {
		Transaction Transaction `json:"transaction"`
		Decision    string      `json:"decision"`
	}

	// Make RPC call to Shield3 here
	var err error
	var client *rpc.Client
	if client == nil {
		retryErr := utils.Retry(3, 1*time.Second, func() error {
			client, err = rpc.Dial(rpcUrl)
			return err
		})
		if retryErr != nil {
			return false, fmt.Errorf("failed to connect to RPC client: %v", err)
		}
	}

	var result json.RawMessage
	err = client.Call(&result, "eth_backfillTransaction", txHash)
	if err != nil {
		return false, fmt.Errorf("failed to backfill transaction: %v", err)
	}

	var backfillTransaction Response
	err = json.Unmarshal(result, &backfillTransaction)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if backfillTransaction.Decision == "Allow" {
		return true, nil
	}

	return false, nil
}
