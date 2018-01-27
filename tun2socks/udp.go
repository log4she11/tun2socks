package tun2socks

import (
	"log"
	"net"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/util"
)

// NewUDPEndpointAndListenIt create a UDP endpoint, bind it, then start read.
func (app *App) NewUDPEndpointAndListenIt() {
	var wq waiter.Queue
	ep, e := app.S.NewEndpoint(udp.ProtocolNumber, app.NetworkProtocolNumber, &wq)
	if e != nil {
		log.Fatal("New UDP Endpoint failed", e)
	}
	defer ep.Close()
	if err := ep.Bind(tcpip.FullAddress{NICId, "", app.HookPort}, nil); err != nil {
		log.Fatal("Bind failed", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		var localAddr tcpip.FullAddress
		v, err := ep.Read(&localAddr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}
			if !util.IsClosed(err) {
				log.Println("[error] read from netstack failed", err)
			}
			udp.UDPNatList.Delete(localAddr.Port)
			continue
		}

		endpointInterface, ok := udp.UDPNatList.Load(localAddr.Port)
		if !ok {
			udp.UDPNatList.Delete(localAddr.Port)
			continue
		}
		endpoint := endpointInterface.(stack.TransportEndpointID)
		// TODO ipv6
		remoteHost := endpoint.LocalAddress.To4().String()
		contains, _ := IgnoreRanger.Contains(net.ParseIP(remoteHost))
		if contains {
			continue
		}

		udpTunnel, existFlag, e := NewUDPTunnel(endpoint, localAddr, app)
		if e != nil {
			log.Println("[error] NewUDPTunnel failed", e)
			udp.UDPNatList.Delete(localAddr.Port)
			continue
		}
		go udpTunnel.Run(v, existFlag)
	}
}