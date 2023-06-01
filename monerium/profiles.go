package monerium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// GetAuthContext retrieves context of authenticated user.
func (c *Client) GetAuthContext(ctx context.Context) (*AuthContext, error) {
	path := "/auth/context"
	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var ac AuthContext
	if err = json.Unmarshal(bs, &ac); err != nil {
		return nil, err
	}

	return &ac, nil
}

// AuthContext represents the context of authenticated user.
type AuthContext struct {
	UserID           string        `json:"userId"`
	Email            string        `json:"email"`
	Name             string        `json:"name"`
	Roles            []string      `json:"roles"`
	Auth             Auth          `json:"auth"`
	DefaultProfileID string        `json:"defaultProfile"`
	Profiles         []AuthProfile `json:"profiles"`
}

type Auth struct {
	Method   string `json:"method"`
	Subject  string `json:"subject"`
	Verified bool   `json:"verified"`
}

type AuthProfile struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Name  string   `json:"name"`
	Perms []string `json:"perms"`
}

// GetProfiles retrieves all profiles summaries.
// The summary contains information about the profile such as its kind and the permission the authenticated user has on the profiles.
func (c *Client) GetProfiles(ctx context.Context) ([]*ProfileSummary, error) {
	path := "/profiles"
	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var ps []*ProfileSummary
	if err = json.Unmarshal(bs, &ps); err != nil {
		return nil, err
	}

	return ps, nil
}

// GetProfile retrieves a single profile details.
func (c *Client) GetProfile(ctx context.Context, req *GetProfileRequest) (*Profile, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/profiles/%s", req.ProfileID)
	bs, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var pr Profile
	if err = json.Unmarshal(bs, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

type GetProfileRequest struct {
	ProfileID string
}

func (r *GetProfileRequest) Validate() error {
	if r == nil {
		return errors.New("GetProfileRequest is required")
	}
	if r.ProfileID == "" {
		return errors.New("empty profileID")
	}

	return nil
}

// ProfileSummary contains auth related information about the profile: type and permissions.
type ProfileSummary struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Permissions []string `json:"perms,omitempty"`
}

// Profile contains general information about the profile: KYC details and linked accounts.
type Profile struct {
	ID       string     `json:"id,omitempty"`
	Name     string     `json:"name,omitempty"`
	KYC      KYCDetails `json:"kyc,omitempty"`
	Accounts []Account  `json:"accounts,omitempty"`
}

// AddAddressToProfile links given blockchain address (wallet) and create an account for Monerium tokens.
func (c *Client) AddAddressToProfile(ctx context.Context, req *AddAddressToProfileRequest) (*Profile, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/profiles/%s/addresses", req.ProfileID)
	bs, err := c.post(ctx, path, req)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err = json.Unmarshal(bs, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

type AddAddressToProfileRequest struct {
	ProfileID string    `json:"-"`
	Address   string    `json:"address"`
	Message   string    `json:"message"`
	Signature string    `json:"signature"`
	Accounts  []Account `json:"accounts"`
}

func (r *AddAddressToProfileRequest) Validate() error {
	if r == nil {
		return errors.New("AddAddressToProfileRequest is required")
	}
	if r.ProfileID == "" {
		return errors.New("empty profileID")
	}

	return nil
}

// KYCDetails represents KYC details of a profile.
type KYCDetails struct {
	State   KYCState `json:"state,omitempty"`
	Outcome string   `json:"outcome,omitempty"`
}

// KYCState represents the state of the customer onboarding.
type KYCState string

const (
	//KYCStateAbsent means there is no KYC version available.
	KYCStateAbsent KYCState = "absent"
	// KYCStateSubmitted means the user has submitted KYC data, but it has not been processed.
	KYCStateSubmitted KYCState = "submitted"
	// KYCStatePending means the admin has started processing the KYC application.
	KYCStatePending KYCState = "pending"
	// KYCStateConfirmed means an admin has decided on the outcome of a KYC application.
	KYCStateConfirmed KYCState = "confirmed"
)

// KYCOutcome represents the verdict of the KYC from Monerium.
type KYCOutcome string

const (
	// KYCOutcomeApproved means a valid customer.
	KYCOutcomeApproved KYCOutcome = "approved"
	// KYCOutcomeRejected mean that the applicant did not meet the KYC requirements.
	KYCOutcomeRejected KYCOutcome = "rejected"
	// KYCOutcomeUnknown the outcome has not been reached yet.
	KYCOutcomeUnknown = "unknown"
)

// Account represents an account in Monerium system.
type Account struct {
	Address       string   `json:"address,omitempty"`
	Chain         Chain    `json:"chain,omitempty"`
	Network       Network  `json:"network,omitempty"`
	Currency      Currency `json:"currency,omitempty"`
	Standard      string   `json:"standard,omitempty"`
	IBAN          string   `json:"iban,omitempty"`
	State         string   `json:"state,omitempty"`
	SortCode      string   `json:"sortCode,omitempty"`
	AccountNumber string   `json:"accountNumber,omitempty"`
}
