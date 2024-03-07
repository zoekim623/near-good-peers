package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Peers []Peer

func (p Peers) Len() int {
	return len(p)
}

func (p Peers) Less(i, j int) bool {
	return p[i].Speed < p[j].Speed
}

func (p Peers) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type Peer struct {
	AccountId string `json:"account_id"`
	Addr      string `json:"addr"`
	Id        string `json::"id"`
	Speed     time.Duration
}

type NetworkResult struct {
	ActivePeers []Peer `json:"active_peers"`
}

type NetworkInfo struct {
	Result NetworkResult `json:"result"`
}

func main() {
	rpc := flag.String("rpc", "http://localhost:3030", "rpc url")
	n := flag.Int("n", 30, "number of peers")
	ms := flag.Int("ms", 1000, "maximum time(ms)")

	flag.Parse()
	ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
	fmt.Println("rpc: ", *rpc)
	peers, err := getPeers(ctx, *rpc)
	if err != nil {
		panic("failed to fetch peers info")
	}

	var goodPeers Peers
	var slowPeers Peers
	var badPeers Peers

	for _, peer := range peers {
		speed, err := checkPeerSpeed(peer.Addr)
		if err != nil {
			fmt.Println("failed to check speed - ", peer.Addr, "error", err)
			badPeers = append(badPeers, peer)
			continue
		}
		peer.Speed = speed

		if speed > time.Duration(*ms)*time.Millisecond {
			fmt.Println("too late peers - ", peer.Addr)
			slowPeers = append(slowPeers, peer)
			continue
		}

		goodPeers = append(goodPeers, peer)
	}
	sort.Sort(goodPeers)

	var selectedPeers []string
	for idx, peer := range goodPeers {
		if idx >= *n {
			break
		}
		fmt.Println("#", idx+1, " ", peer.Addr, " speed: ", peer.Speed)
		selectedPeers = append(selectedPeers, peer.Addr)
	}
	fmt.Println("total actived peers ", len(peers))
	fmt.Println("total bad peer: ", len(badPeers))
	fmt.Println("total slow peer: ", len(slowPeers))
	fmt.Println("total fast peer: ", len(selectedPeers))
	persistentPeers := strings.Join(selectedPeers, ",")
	fmt.Println(persistentPeers)
}

func checkPeerSpeed(url string) (time.Duration, error) {
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", url, 3*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	speed := time.Since(startTime)
	fmt.Printf("url: %s, speed: %d(ms)\n", url, speed/time.Millisecond)
	return speed, nil
}

func getPeers(ctx context.Context, url string) ([]Peer, error) {
	data := `
	{
		"jsonrpc": "2.0",
		"id": "dontcare",
		"method": "network_info",
		"params": []
	}
	`
	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(data)))
	if err != nil {
		select {
		case <-ctx.Done():
			fmt.Println("request canceled or time out", ctx.Err())
			return nil, ctx.Err()
		default:
			fmt.Println("failed to fetch netinfo", err)
			return nil, err
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to read response body", err)
		return nil, err
	}

	var networkInfo NetworkInfo
	err = json.Unmarshal(body, &networkInfo)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil, err
	}
	return networkInfo.Result.ActivePeers, nil
}
