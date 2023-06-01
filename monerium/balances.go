package monerium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// GetBalancesForProfile retrieves balance for every account of a profile.
// Each account represent one token, on a chain and network.
func (c *Client) GetBalancesForProfile(ctx context.Context, req *GetBalancesForProfileRequest) ([]*ProfileBalance, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/profiles/%s/balances", req.ProfileID)
	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var pbs []*ProfileBalance
	if err = json.Unmarshal(bs, &pbs); err != nil {
		return nil, err
	}
	return pbs, nil
}

// GetBalancesForProfileRequest contains data needed for making the request.
type GetBalancesForProfileRequest struct {
	ProfileID string
}

// Validate checks GetBalancesForProfileRequest.
func (r *GetBalancesForProfileRequest) Validate() error {
	if r == nil {
		return errors.New("GetBalancesForProfileRequest is required")
	}

	return nil
}

// GetBalances retrieves balance for every account of the default profile.
// Each account represent one token, on a chain and network.
func (c *Client) GetBalances(ctx context.Context) ([]*ProfileBalance, error) {
	path := "/balances"

	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var pbs []*ProfileBalance
	if err = json.Unmarshal(bs, &pbs); err != nil {
		return nil, err
	}

	return pbs, nil
}

// GetTokens retrieves information about the emoney tokens with tickers, symbols, decimals, token contract
// address and the network and chain information, we currently support Ethereum and Polygon.
func (c *Client) GetTokens(ctx context.Context) ([]*Token, error) {
	path := "/tokens"

	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var ts []*Token
	if err = json.Unmarshal(bs, &ts); err != nil {
		return nil, err
	}

	return ts, nil
}

// ProfileBalance represents balances of a profile identified by ProfileID.
type ProfileBalance struct {
	ProfileID string     `json:"id,omitempty"`
	Address   string     `json:"address,omitempty"`
	Chain     string     `json:"chain,omitempty"`
	Network   string     `json:"network,omitempty"`
	Balances  []*Balance `json:"balances,omitempty"`
}

// Balance represents a balance - amount and currency.
type Balance struct {
	Amount   string `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// Token represents an e-money token: its chain, network, address and so on.
type Token struct {
	Currency Currency `json:"currency,omitempty"`
	Ticker   Ticker   `json:"ticker,omitempty"`
	Symbol   Symbol   `json:"symbol,omitempty"`
	Chain    Chain    `json:"chain,omitempty"`
	Network  Network  `json:"network,omitempty"`
	Address  string   `json:"address,omitempty"`
	Decimals uint     `json:"decimals,omitempty"`
}

type Symbol string

const (
	SymbolEURe Symbol = "EURe"
	SymbolUSDe Symbol = "USDe"
	SymbolGBPe Symbol = "GBPe"
	SymbolISKe Symbol = "ISKe"
)

type Ticker string

const (
	TickerEUR Ticker = "EUR"
	TickerUSD Ticker = "USD"
	TickerGBP Ticker = "GBP"
	TickerISK Ticker = "ISK"
)

type Currency string

const (
	CurrencyEUR Currency = "eur"
	CurrencyUSD Currency = "usd"
	CurrencyGBP Currency = "gbp"
	CurrencyISK Currency = "isk"
)
