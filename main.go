package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

type ipInfo struct {
	Ip       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
	Readme   string `json:"readme"`
}

func main() {
	url := "https://ipinfo.io"

	domain, e := os.LookupEnv("DOMAIN")
	if !e {
		log.Fatal("env \"DOMAIN\" is required!!!")
	}

	hostedZoneId, e := os.LookupEnv("HOSTED_ZONE_ID")
	if !e {
		log.Fatal("env \"HOSTED_ZONE_ID\" is required!!!")
	}

	// ipinfo.ioのAPIを叩く
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var info ipInfo

	if err := json.Unmarshal(body, &info); err != nil {
		log.Fatal(err)
	}

	// AWSのAPI叩く
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	)
	if err != nil {
		fmt.Println(err.Error())
	}

	svc := route53.New(sess)
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    &hostedZoneId, // Required
		StartRecordName: &domain,
		StartRecordType: aws.String(route53.RRTypeA),
	}

	var registeredIp string

	result, err := svc.ListResourceRecordSets(input)
	if err != nil {
		log.Fatal(err.Error())
	}
	for _, record := range result.ResourceRecordSets {
		if *record.Name == domain && *record.Type == route53.RRTypeA {
			registeredIp = *record.ResourceRecords[0].Value
		}
	}

	if info.Ip == registeredIp {
		fmt.Println("IP address has not changed.")
	} else {
		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: &hostedZoneId,
			ChangeBatch: &route53.ChangeBatch{
				Changes: []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(domain),
							ResourceRecords: []*route53.ResourceRecord{
								{
									Value: aws.String(info.Ip),
								},
							},
							TTL:  aws.Int64(300),
							Type: aws.String(route53.RRTypeA),
						},
					},
				},
			},
		}
		_, err := svc.ChangeResourceRecordSets(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case route53.ErrCodeNoSuchHostedZone:
					fmt.Println(route53.ErrCodeNoSuchHostedZone, aerr.Error())
				case route53.ErrCodeNoSuchHealthCheck:
					fmt.Println(route53.ErrCodeNoSuchHealthCheck, aerr.Error())
				case route53.ErrCodeInvalidChangeBatch:
					fmt.Println(route53.ErrCodeInvalidChangeBatch, aerr.Error())
				case route53.ErrCodeInvalidInput:
					fmt.Println(route53.ErrCodeInvalidInput, aerr.Error())
				case route53.ErrCodePriorRequestNotComplete:
					fmt.Println(route53.ErrCodePriorRequestNotComplete, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return
		}
		fmt.Println("Updated IP Address. New Address is \"" + info.Ip + "\"")
	}
}
