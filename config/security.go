package config

import (
	"fmt"
	"reflect"

	security "github.com/libp2p/go-conn-security"
	csms "github.com/libp2p/go-conn-security-multistream"
	insecure "github.com/libp2p/go-conn-security/insecure"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	secio "github.com/libp2p/go-libp2p-secio"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
)

// SecC is a security transport constructor
type SecC func(h host.Host) (security.Transport, error)

// MsSecC is a tuple containing a security transport constructor and a protocol
// ID.
type MsSecC struct {
	SecC
	ID string
}

var securityArgTypes = map[reflect.Type]constructor{
	hostType:    func(h host.Host, _ *tptu.Upgrader) interface{} { return h },
	networkType: func(h host.Host, _ *tptu.Upgrader) interface{} { return h.Network() },
	peerIDType:  func(h host.Host, _ *tptu.Upgrader) interface{} { return h.ID() },
	privKeyType: func(h host.Host, _ *tptu.Upgrader) interface{} { return h.Peerstore().PrivKey(h.ID()) },
	pubKeyType:  func(h host.Host, _ *tptu.Upgrader) interface{} { return h.Peerstore().PubKey(h.ID()) },
	pstoreType:  func(h host.Host, _ *tptu.Upgrader) interface{} { return h.Peerstore() },
}

// SecurityConstructor creates a security constructor from the passed parameter
// using reflection.
func SecurityConstructor(sec interface{}) (SecC, error) {
	// Already constructed?
	if t, ok := sec.(security.Transport); ok {
		return func(_ host.Host) (security.Transport, error) {
			return t, nil
		}, nil
	}

	fn, err := makeConstructor(sec, securityType, securityArgTypes)
	if err != nil {
		return nil, err
	}
	return func(h host.Host) (security.Transport, error) {
		t, err := fn(h, nil)
		if err != nil {
			return nil, err
		}
		return t.(security.Transport), nil
	}, nil
}

func makeInsecureTransport(id peer.ID) security.Transport {
	secMuxer := new(csms.SSMuxer)
	secMuxer.AddTransport(insecure.ID, insecure.New(id))
	return secMuxer
}

func makeSecurityTransport(h host.Host, tpts []MsSecC) (security.Transport, error) {
	secMuxer := new(csms.SSMuxer)
	if len(tpts) > 0 {
		transportSet := make(map[string]struct{}, len(tpts))
		for _, tptC := range tpts {
			if _, ok := transportSet[tptC.ID]; ok {
				return nil, fmt.Errorf("duplicate security transport: %s", tptC.ID)
			}
		}
		for _, tptC := range tpts {
			tpt, err := tptC.SecC(h)
			if err != nil {
				return nil, err
			}
			if _, ok := tpt.(*insecure.Transport); ok {
				return nil, fmt.Errorf("cannot construct libp2p with an insecure transport, set the Insecure config option instead")
			}
			secMuxer.AddTransport(tptC.ID, tpt)
		}
	} else {
		id := h.ID()
		sk := h.Peerstore().PrivKey(id)
		secMuxer.AddTransport(secio.ID, &secio.Transport{
			LocalID:    id,
			PrivateKey: sk,
		})
	}
	return secMuxer, nil
}
