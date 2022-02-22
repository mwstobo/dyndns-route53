package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	lastSuccessfulExecution = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_successful_execution_timestamp_seconds",
		Help: "Timestamp of last successful execution of dyndns-route53",
	})
)

func getCurrentIP() (string, error) {
	resp, err := http.Get("https://ifconfig.co")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := string(body)
	log.Print("Current IP is ", ip)
	return strings.TrimSpace(ip), nil
}

func getCurrentIPForHost(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", nil
	}

	ip := ips[0].String()
	log.Print("Current IP for ", host, " is ", ip)
	return strings.TrimSpace(ip), nil
}

func updateDNS(
	ctx context.Context, client *route53.Client, hostedZoneId string,
	ip string, host string, ttl int64) error {

	input := route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &hostedZoneId,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: &host,
						Type: types.RRTypeA,
						ResourceRecords: []types.ResourceRecord{
							{
								Value: &ip,
							},
						},
						TTL: &ttl,
					},
				},
			},
		},
	}

	if _, err := client.ChangeResourceRecordSets(ctx, &input); err != nil {
		return err
	}

	log.Print("Updated ", host, " to ", ip)
	return nil
}

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal("error loading default AWS config: ", err)
	}
	client := route53.NewFromConfig(cfg)

	hostedZoneId := flag.String("hosted-zone-id", "", "hostedZoneID to use when creating route53 records")
	host := flag.String("host", "", "name to associate to your home IP address")
	ttl := flag.Int64("ttl", int64(300), "TTL for you DNS record")
	pushGateway := flag.String("push-gateway", "http://localhost:9091", "URL for a Prometheus pushgateway")
	flag.Parse()

	push := push.New(*pushGateway, "dyndns_route53").Collector(lastSuccessfulExecution)

	ip, err := getCurrentIP()
	if err != nil {
		log.Fatal("error getting current IP: ", err)
	} else if strings.TrimSpace(ip) == "" {
		log.Fatal("found blank current IP")
	}

	hostIp, err := getCurrentIPForHost(*host)
	if err != nil {
		log.Fatalf("error getting IP for %s: %v", *host, err)
	} else if strings.TrimSpace(hostIp) == "" {
		log.Fatalf("found blank IP for %s", *host)
	}

	if hostIp != ip {
		if err := updateDNS(ctx, client, *hostedZoneId, ip, *host, *ttl); err != nil {
			log.Fatal("error updating DNS: ", err)
		}
	}

	lastSuccessfulExecution.Set(float64(time.Now().Unix()))
	push.Push()
}
