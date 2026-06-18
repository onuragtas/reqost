package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
)

// Minimal Postman v2.1 structs for generation only.
type Collection struct {
	Info CollectionInfo `json:"info"`
	Item []Item         `json:"item"`
}

type CollectionInfo struct {
	ID     string `json:"_postman_id"`
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type Item struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Item    []Item   `json:"item,omitempty"`
	Request *Request `json:"request,omitempty"`
}

type Request struct {
	Method string     `json:"method"`
	URL    RequestURL `json:"url"`
	Header []Header   `json:"header,omitempty"`
	Body   *Body      `json:"body,omitempty"`
}

type RequestURL struct {
	Raw string `json:"raw"`
}

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Body struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

var (
	methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	hosts   = []string{"api.acme.io", "gateway.prod", "backend.internal", "svc.k8s"}
	paths   = []string{"users", "orders", "products", "payments", "inventory", "auth", "reports", "events"}
)

func main() {
	// Structure: 50 root folders × 20 sub-folders × 49 requests
	// Total: 50 + 1000 + 49000 = 50,050 items
	const (
		rootCount = 50
		subCount  = 20
		leafCount = 49
	)

	rng := rand.New(rand.NewSource(42))

	col := Collection{
		Info: CollectionInfo{
			ID:     "reqost-seed-50k",
			Name:   "50k Benchmark Collection",
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
	}

	for i := 0; i < rootCount; i++ {
		root := Item{
			ID:   fmt.Sprintf("r%d", i),
			Name: fmt.Sprintf("Service %02d — %s", i, paths[i%len(paths)]),
		}
		for j := 0; j < subCount; j++ {
			sub := Item{
				ID:   fmt.Sprintf("r%d-s%d", i, j),
				Name: fmt.Sprintf("Module %02d", j),
			}
			for k := 0; k < leafCount; k++ {
				method := methods[rng.Intn(len(methods))]
				host := hosts[rng.Intn(len(hosts))]
				path := paths[rng.Intn(len(paths))]
				sub.Item = append(sub.Item, Item{
					ID:   fmt.Sprintf("r%d-s%d-q%d", i, j, k),
					Name: fmt.Sprintf("%s /%s/%d", method, path, k),
					Request: &Request{
						Method: method,
						URL:    RequestURL{Raw: fmt.Sprintf("https://%s/api/v1/%s/%d", host, path, k)},
						Header: []Header{
							{Key: "Content-Type", Value: "application/json"},
							{Key: "Authorization", Value: "Bearer {{token}}"},
						},
						Body: &Body{
							Mode: "raw",
							Raw:  fmt.Sprintf(`{"id":%d,"name":"item %d","service":%d}`, k, k, i),
						},
					},
				})
			}
			root.Item = append(root.Item, sub)
		}
		col.Item = append(col.Item, root)
	}

	outPath := "collection-50k.json"
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(col); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	total := rootCount + rootCount*subCount + rootCount*subCount*leafCount
	fmt.Printf("Generated %d items → %s\n", total, outPath)
}
