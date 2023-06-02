# monerium/go-sdk

go-sdk is a Go client library for accessing the [Monerium API](https://monerium.dev/api-docs).

## Installation

Monerium's go-sdk is compatible with standard Go modules workflow.  
To install:

```
go get github.com/monerium/go-sdk
```

## Usage

### Authentication

While Monerium API in general supports a few ways to authenticate, the go-sdk supports only OAuth2 Client Credentials flow.

In order to receive credentials one first needs to register itself to Monerium via:
- https://monerium.app for production
- https://sandbox.monerium.dev for sandbox

Next, one can connect the wallet, complete the KYC and request a new IBAN.

Having those steps completed one navigates to "Developers" section and registers a new app to obtain API credentials (client_id and client_secret).
Providing Redirect URL is not needed for Client Credentials flow.

### Getting started

Provided that you obtained the API credentials and bootstrap some Go application let's initialize the SDK:

```go
c := monerium.NewClient(
	context.Background(),
	monerium.SandboxBaseURL,
	monerium.SandboxWebsocketURL,
	&monerium.AuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     monerium.SandboxTokenURL,
	})
```

The SDK provides helpful constants for Sandbox and Production URLs.

Next, we'll confirm that the connection works by getting AuthContext, to get information about authenticated user (yes, that's you!).

```go
ac, err := c.GetAuthContext(context.Background())
if err != nil {
	log.Fatal(err)
}
fmt.Println("Your default Profile ID is: ", ac.DefaultProfileID)
```

What you just got in the response is the ID of the default profile associated with your account.   
This ID can be later used to connecting new wallets to the profile, placing new orders and more.

### Placing a new order

Assuming that you have a ready to use IBAN and there are some funds there (issued by sending money to the IBAN over SEPA) we can place a new redeem order.

Besides obvious things like amount, currency and counterpart of the order, one also needs
to submit a message and signature - the message signed with a private key of your account.

The message is composed as such: `Send <CURRENCY> <AMOUNT> to <IBAN> at <TIMESTAMP>`.  
Timestamp is formatted as RFC3339 and needs to be accurate to the minute.

Assuming we have your private key ready in the code (as *ecdsa.PrivateKey), let's create the message and its signature:

```go
import (
    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/ethereum/go-ethereum/crypto"
)

// ...

var (
    msg  = "Send EUR 1 to GR1601101250000000012300695 at " + time.Now().Format(time.RFC3339)
    pk   = privateKeyFrom(privateKeyStr)
    sig  = signatureFrom(msg, pk)
    addr = "0x123" // address associated with private key
)

// ...

func privateKeyFrom(s string) (*ecdsa.PrivateKey, error) {
    bs, err := hexutil.Decode(s)
    if err != nil {
        return nil, err
    }
	
	return crypto.ToECDSAUnsafe(bs), nil
}

func signatureFrom(msg string, pk *ecdsa.PrivateKey) (string, error) {
	data := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(msg), msg)
	hash := crypto.Keccak256Hash([]byte(data))
	signature, err := crypto.Sign(hash.Bytes(), pk)
	if err != nil {
		return nil, err
	}

	return hexutil.Encode(signature)
}
```

Having those steps completed all the hard work is done, we can construct the request:

```go
o, err := c.PlaceOrder(ctx, &monerium.PlaceOrderRequest{
	Address:   addr, 
	Currency:  monerium.CurrencyEUR,
	Chain:     monerium.ChainGnosis,
	Kind:      monerium.OrderKindRedeem,
	Amount:    "1",
	Message:   msg,
	Signature: sig,
	Counterpart: &monerium.Counterpart{
		Identifier: monerium.Identifier{
			Standard: "iban",
			IBAN:     "GR1601101250000000012300695",
		},
		Details: monerium.CounterpartDetails{
			Country:   "GR",
			FirstName: "Test",
			LastName:  "Testsson",
		},
	},
})
if err != nil {
	log.Fatal(err)
}
```

and voilà!

### Listening for new orders

Another core use-case of the Monerium API is ability to subscribe to a WebSocket for order notifications.
It's useful because the placed order is not processed immediately, and we might want to act on the confirmation. Also, we might be interested in triggering
some actions based on the incoming transfers to your IBAN (those are the 'issue' orders).

In order to use OrdersNotifications call, one needs to create a cancellable context and channel to receive OrderResults.
The ProfileID in the request can be obtained for example from AuthContext call.

```go
ctx, cancel := context.WithCancel(context.Background())

orders := make(chan *monerium.OrderResult)
defer close(orders)

err := c.OrdersNotifications(ctx,
	&monerium.OrdersNotificationsRequest{
	    ProfileID: defaultProfileID,
	},
	orders)
if err != nil {
	t.Fatal(err)
}

for {
	o := <-orders
	if o.Error != nil {
		fmt.Printf("Error: %+v\n", o.Error)
		continue
	}
	if o.Order != nil {
		fmt.Printf("Order: %+v\n", o.Order)
		if o.Order.Meta.State == monerium.OrderStateProcessed || o.Order.Meta.State == monerium.OrderStateRejected {
			cancel()
			return
		}
	}
}
```

The snippet above listens for new orders and cancels the connection on the first processed or rejected order.  
I believe, you should be good to go from here.

Good luck!