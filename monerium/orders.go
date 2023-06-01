package monerium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-querystring/query"
	"nhooyr.io/websocket"
)

// PlaceOrder initialize a payment to an external SEPA account (redeem order).
//
// The payload includes the amount, currency and the beneficiary (counterpart).
// All SEPA payments must be authorized using a strong customer authentication.
// In short, users must provide two of three elements to authorize payments:
//   - Knowledge: something only the user knows, e.g. a password or a PIN code
//   - Possession: something only the user possesses, e.g. a mobile phone
//   - Inherence: something the user is, e.g. the use of a fingerprint or voice recognition.
//
// The authorization is implemented by requiring a signature derived from a private key (possession) in addition to a password (knowledge).
// A message, the signature and the address associated with the private key used to sign must be added to the request payload.
func (c *Client) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (*Order, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := "/orders"
	bs, err := c.post(ctx, path, req)
	if err != nil {
		return nil, err
	}
	var o Order
	if err = json.Unmarshal(bs, &o); err != nil {
		return nil, err
	}

	return &o, nil
}

// PlaceOrderRequest contains parameters for placing an order.
// Order can be placed either with set of Address, Currency and Chain or AccountID.
// Memo is a reference of the SEPA transfer.
// SupportingDocumentID is a document to be attached for redeem order above certain limit.
// Memo and SupportingDocumentID are optional.
//
// SupportingDocumentID is the ID of a uploaded file via UploadFile call.
type PlaceOrderRequest struct {
	Address   string   `json:"address,omitempty"`
	Currency  Currency `json:"currency,omitempty"`
	Chain     Chain    `json:"chain,omitempty"`
	AccountID string   `json:"accountId,omitempty"`

	Kind        OrderKind    `json:"kind"`
	Amount      string       `json:"amount"`
	Signature   string       `json:"signature"`
	Message     string       `json:"message"`
	Counterpart *Counterpart `json:"counterpart"`

	Memo                 string `json:"memo,omitempty"`
	SupportingDocumentID string `json:"supportingDocumentId,omitempty"`
}

// Validate checks if PlaceOrderRequest is correct.
func (r *PlaceOrderRequest) Validate() error {
	if r == nil {
		return errors.New("PlaceOrderRequest is required")
	}
	if r.Kind != OrderKindRedeem {
		return errors.New("only redeem order is possible to be placed")
	}
	if r.Counterpart == nil {
		return errors.New("order counterpart is missing")
	}
	if r.Message == "" || r.Signature == "" {
		return errors.New("message or signature missing")
	}

	if r.AccountID != "" {
		return nil
	}
	if r.Chain == "" || r.Currency == "" || r.Address == "" {
		return errors.New("either AccountID or Chain, Address and Currency are required")
	}

	return nil
}

// Order represents a payment Order.
// If order is rejected, the reason is stored in RejectedReason.
type Order struct {
	ID                   string      `json:"id,omitempty"`
	Profile              string      `json:"profile,omitempty"`
	AccountID            string      `json:"accountId,omitempty"`
	Address              string      `json:"address,omitempty"`
	Kind                 OrderKind   `json:"kind,omitempty"`
	Amount               string      `json:"amount,omitempty"`
	Currency             Currency    `json:"currency,omitempty"`
	Counterpart          Counterpart `json:"counterpart,omitempty"`
	Memo                 string      `json:"memo,omitempty"`
	RejectedReason       string      `json:"rejectedReason,omitempty"`
	SupportingDocumentID string      `json:"supportingDocumentId,omitempty"`
	Meta                 OrderMeta   `json:"meta,omitempty"`
}

// GetOrders retrieves all orders accessible by the authenticated user.
// Query parameters passed in GetOrderRequest can be used to filter and sort the result.
// GetOrderRequest can be nil, in that case no filters are applied.
func (c *Client) GetOrders(ctx context.Context, req *GetOrdersRequest) ([]*Order, error) {
	path := "/orders"
	if req != nil {
		v, err := query.Values(req)
		if err != nil {
			return nil, err
		}
		path = "/orders?" + v.Encode()
	}

	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var os []*Order
	if err = json.Unmarshal(bs, &os); err != nil {
		return nil, err
	}

	return os, nil
}

// GetOrdersRequest contains optional query parameters that can be used to filter results.
type GetOrdersRequest struct {
	Address   string     `url:"address"`
	TxHash    string     `url:"txHash"`
	Memo      string     `url:"memo"`
	State     OrderState `url:"state"`
	AccountID string     `url:"accountId"`
	ProfileID string     `url:"profile"`
}

// GetOrder retrieves order based on OrderID.
func (c *Client) GetOrder(ctx context.Context, req *GetOrderRequest) (*Order, error) {
	path := fmt.Sprintf("/orders/%s", req.OrderID)

	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var o *Order
	if err = json.Unmarshal(bs, &o); err != nil {
		return nil, err
	}

	return o, nil
}

// GetOrderRequest contains optional query parameters that can be used to filter results.
type GetOrderRequest struct {
	OrderID string `url:"orderId"`
}

// OrdersNotifications streams order updates over a channel.
//
// The websocket will emit the same order object up to three times, once for the following state changes:
// 1. Placed - the initial state of placed order.
// 2. Pending - order is being processed.
// 3. Processed - money has been received for issue orders or tokens have been burnt for redeem orders.
//
// Pending state is optional and Order might transform from placed straight to processed.
// OrderResult contains Order on sucessfull response or Error on failure.
func (c *Client) OrdersNotifications(ctx context.Context, req *OrdersNotificationsRequest, os chan<- *OrderResult) error {
	tok, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	path := c.wsURL + "/orders"
	if req != nil && req.ProfileID != "" {
		path = fmt.Sprintf("%s/profiles/%s/orders", c.wsURL, req.ProfileID)
	}

	wc, err := dialWebsocket(ctx, path, tok)
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %w", err)
	}

	ticker := time.NewTicker(c.notifyTick)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wc.Close(websocket.StatusNormalClosure, "stopping connection")
				os <- &OrderResult{nil, ctx.Err()}

				return
			case <-ticker.C:
				o, err := readOrder(ctx, wc)
				if err != nil {
					os <- &OrderResult{nil, fmt.Errorf("failed to read order: %w", err)}
				}

				os <- &OrderResult{o, nil}
			}
		}
	}()

	return nil
}

// OrdersNotificationsRequest represents request data fro Order notifications.
type OrdersNotificationsRequest struct {
	ProfileID string
}

// OrderResult contains Order response on success or Error with failure reason.
type OrderResult struct {
	Order *Order
	Error error
}

// OrderKind represents Order kind.
// Only redeem order can be placed via API.
// Issue orders are created via money transfer over SEPA to IBAN number provided by Monerium.
type OrderKind string

const (
	OrderKindRedeem OrderKind = "redeem"
	OrderKindIssue  OrderKind = "issue"
)

// OrderKind represents Order kind.
type OrderState string

const (
	OrderStatePlaced    OrderState = "placed"
	OrderStatePending   OrderState = "pending"
	OrderStateProcessed OrderState = "processed"
	OrderStateRejected  OrderState = "rejected"
)

// OrderMeta represents the metadata of an Order.
type OrderMeta struct {
	ApprovedAt     time.Time  `json:"approvedAt,omitempty"`
	ProcessedAt    time.Time  `json:"processedAt,omitempty"`
	RejectedAt     time.Time  `json:"rejectedAt,omitempty"`
	State          OrderState `json:"state,omitempty"`
	PlacedBy       string     `json:"placedBy,omitempty"`
	PlacedAt       time.Time  `json:"placedAt,omitempty"`
	ReceivedAmount string     `json:"receivedAmount,omitempty"`
	SentAmount     string     `json:"sentAmount,omitempty"`
}

// Counterpart represents the counterpart of an Order.
type Counterpart struct {
	Identifier Identifier         `json:"identifier,omitempty"`
	Details    CounterpartDetails `json:"details,omitempty"`
}

// Identifier represents the identifier of a Counterpart.
type Identifier struct {
	Standard string `json:"standard,omitempty"`
	IBAN     string `json:"iban,omitempty"`
}

// CounterpartDetails represents the details of a Counterpart.
type CounterpartDetails struct {
	Country   string `json:"country,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}

// Chain represents supported blockchains.
type Chain string

const (
	ChainEthereum Chain = "ethereum"
	ChainPolygon  Chain = "polygon"
	ChainGnosis   Chain = "gnosis"
)

// Network represents supported blockchain networks.
type Network string

const (
	NetworkMainnet Network = "mainnet"
	NetworkGoerli  Network = "goerli"
	NetworkMumbai  Network = "mumbai"
	NetworkChiado  Network = "chiado"
)

// readOrder reads Order from websocket connection.
func readOrder(ctx context.Context, conn *websocket.Conn) (*Order, error) {
	mt, bs, err := conn.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read from websocket: %w", err)
	}
	if mt != websocket.MessageText {
		return nil, fmt.Errorf("unsupported message type: %s", mt)
	}
	o, err := newOrderFrom(bs)
	if err != nil {
		return nil, fmt.Errorf("failed to build order: %w", err)
	}

	return o, nil
}

// newOrderFrom returns a new Order from slice of bytes.
func newOrderFrom(bs []byte) (*Order, error) {
	var o Order
	if err := json.Unmarshal(bs, &o); err != nil {
		return nil, err
	}

	return &o, nil
}
