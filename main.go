package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
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

	return string(body), nil
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
						Type: types.RRTypeCname,
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

	_, err := client.ChangeResourceRecordSets(ctx, &input)
	if err != nil {
		return err
	}
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

	ip, err := getCurrentIP()
	if err != nil {
		log.Fatal("error getting current IP: ", err)
	}

	if err := updateDNS(ctx, client, *hostedZoneId, ip, *host, *ttl); err != nil {
		log.Fatal("error updating DNS: ", err)
	}
}
