/*
Copyright © 2019 Allan Hung <hung.allan@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"

	"database/sql"
	"github.com/GoogleCloudPlatform/terraformer/providers/alicloud"
	"github.com/GoogleCloudPlatform/terraformer/providers/alicloud/connectivity"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/spf13/cobra"
)

type ecsInfo struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

type argList []string

func (v *argList) String() string {
	return fmt.Sprint(*v)
}

func (v *argList) Type() string {
	return "argList"
}

func (v *argList) Set(value string) error {
	for _, filePath := range strings.Split(value, ",") {
		*v = append(*v, filePath)
	}
	return nil
}

type ecsCmdFlags struct {
	output       string
	labelPrefix  string
	instanceName string
	pageSize     int
	tag          argList
	regName      argList
	noTagKey     argList
	noTagValue   argList
}

var cmdFlags = ecsCmdFlags{}
var db = &sql.DB{}

const nodeExporterPort = "9100"

// ecsCmd represents the ecs command
var ecsCmd = &cobra.Command{
	Use:   "ecs",
	Short: "ECS Service Discovery for Prometheus",
	Long: `This tool provides Prometheus service discovery for ECS running on Alicloud. 

for example:
  prometheus-ali-sd ecs -l ecs -t cluster=prod --notagk "acs:autoscaling.*" --notagv autoScale --logfile /tmp/promsd.log --loglevel debug -o /tmp/promsd.json`,
	Run: func(cmd *cobra.Command, args []string) {
		var ecsInfoList []ecsInfo
		var ecsInfo ecsInfo
		instanceList := map[string]string{}

		config, err := alicloud.LoadConfigFromProfile("")
		if err != nil {
			logger.Errorln("Load config error:", err)
			os.Exit(1)
		}
		aliclient, err := config.Client()
		if err != nil {
			logger.Errorln("Client create failed:", err)
			os.Exit(1)
		}

		vpcMap, err := getVPCInfo(aliclient)
		if err != nil {
			logger.Errorln("Get vpc infomation error:", err)
			os.Exit(1)
		}

		remaining := 1
		pageNumber := 1
		pageSize := cmdFlags.pageSize

		allInstances := make([]ecs.Instance, 0)
		tags := []ecs.DescribeInstancesTag{}
		for _, tag := range cmdFlags.tag {
			tagList := strings.Split(tag, "=")
			instanceTag := ecs.DescribeInstancesTag{
				Key:   tagList[0],
				Value: tagList[1],
			}
			tags = append(tags, instanceTag)
		}
		for remaining > 0 {
			raw, err := aliclient.WithEcsClient(func(ecsClient *ecs.Client) (interface{}, error) {
				request := ecs.CreateDescribeInstancesRequest()
				request.Tag = &tags
				if cmdFlags.instanceName != "" {
					request.InstanceName = cmdFlags.instanceName
				}
				request.RegionId = aliclient.RegionId
				request.PageSize = requests.NewInteger(pageSize)
				request.PageNumber = requests.NewInteger(pageNumber)
				return ecsClient.DescribeInstances(request)
			})
			if err != nil {
				logger.Errorln("Get ecs information error:", err)
				os.Exit(1)
			}
			response := raw.(*ecs.DescribeInstancesResponse)
			for _, instance := range response.Instances.Instance {
				newLabel := true
				allInstances = append(allInstances, instance)
				// name match
				nameMatch := (len(cmdFlags.regName) == 0)
				for _, regRule := range cmdFlags.regName {
					match, _ := regexp.MatchString(regRule, instance.InstanceName)
					if match {
						nameMatch = true
						logger.Debugf("instance %s is include by name rule: %s", instance.InstanceName, regRule)
						break
					}
				}

				if nameMatch {
					// tag key match
					noTagKeyMatch := true
					for _, regRule := range cmdFlags.noTagKey {
						r, _ := regexp.Compile(regRule)
						for _, tag := range instance.Tags.Tag {
							if r.MatchString(tag.TagKey) {
								noTagKeyMatch = false
								logger.Debugf("instance %s is exclude by no tag key rule: %s", instance.InstanceName, regRule)
								break
							}
						}
						if !noTagKeyMatch {
							break
						}
					}
					if noTagKeyMatch {
						// tag value match
						noTagValueMatch := true
						for _, regRule := range cmdFlags.noTagValue {
							r, _ := regexp.Compile(regRule)
							for _, tag := range instance.Tags.Tag {
								if r.MatchString(tag.TagValue) {
									noTagValueMatch = false
									logger.Debugf("instance %s is exclude by no tag value rule: %s", instance.InstanceName, regRule)
									break
								}
							}
							if !noTagValueMatch {
								break
							}
						}
						if noTagValueMatch {
							instanceName := fmt.Sprintf("%s.%s", instance.InstanceName, "ali-netbase.com")
							if instanceList[instance.InstanceName] != "" {
								logger.Errorf("instance id %s with name %s duplicate with instance id %s", instance.InstanceId, instance.InstanceName, instanceList[instance.InstanceName])
							} else {
								instanceList[instance.InstanceName] = instance.InstanceId
								ecsInfo.Targets = []string{fmt.Sprintf("%s:%s", instanceName, nodeExporterPort)}
								ecsInfo.Labels = map[string]string{"exporter": "node_exporter"}
								for _, tag := range instance.Tags.Tag {
									if cmdFlags.labelPrefix != "" {
										ecsInfo.Labels[fmt.Sprintf("%s_%s", cmdFlags.labelPrefix, tag.TagKey)] = tag.TagValue
									} else {
										ecsInfo.Labels[tag.TagKey] = tag.TagValue
									}
								}
								if cmdFlags.labelPrefix != "" {
									ecsInfo.Labels[fmt.Sprintf("%s_%s", cmdFlags.labelPrefix, "vpc")] = vpcMap[instance.VpcAttributes.VpcId]
								} else {
									ecsInfo.Labels["vpc"] = vpcMap[instance.VpcAttributes.VpcId]
								}
								for index, ecs := range ecsInfoList {
									if reflect.DeepEqual(ecs.Labels, ecsInfo.Labels) {
										ecsInfoList[index].Targets = append(ecsInfoList[index].Targets, fmt.Sprintf("%s:%s", instanceName, nodeExporterPort))
										newLabel = false
										break
									}
								}
								if newLabel {
									ecsInfoList = append(ecsInfoList, ecsInfo)
								}
							}
						}
					}
				}
			}
			remaining = response.TotalCount - pageNumber*pageSize
			pageNumber++
		}
		jsonScrapeConfig, err := json.MarshalIndent(ecsInfoList, "", "\t")
		if err != nil {
			logger.Errorln("Json ERROR:", err)
			os.Exit(1)
		}
		err = ioutil.WriteFile(cmdFlags.output, jsonScrapeConfig, 0644)
		if err != nil {
			logger.Errorln("File output error:", err)
			os.Exit(1)
		}
	},
}

func getVPCInfo(aliClient *connectivity.AliyunClient) (map[string]string, error) {
	vpcMap := map[string]string{}

	remaining := 1
	pageNumber := 1
	pageSize := cmdFlags.pageSize

	allVpcs := make([]vpc.Vpc, 0)

	for remaining > 0 {
		raw, err := aliClient.WithVpcClient(func(vpcClient *vpc.Client) (interface{}, error) {
			request := vpc.CreateDescribeVpcsRequest()
			request.RegionId = aliClient.RegionId
			request.PageSize = requests.NewInteger(pageSize)
			request.PageNumber = requests.NewInteger(pageNumber)
			return vpcClient.DescribeVpcs(request)
		})
		if err != nil {
			return vpcMap, err
		}

		response := raw.(*vpc.DescribeVpcsResponse)
		for _, Vpc := range response.Vpcs.Vpc {
			vpcMap[Vpc.VpcId] = Vpc.VpcName
			allVpcs = append(allVpcs, Vpc)
		}
		remaining = response.TotalCount - pageNumber*pageSize
		pageNumber++
	}

	return vpcMap, nil
}

func init() {
	rootCmd.AddCommand(ecsCmd)
	f := ecsCmd.Flags()
	f.StringVarP(&cmdFlags.output, "output", "o", "/tmp/test.json", "file output path")
	f.StringVarP(&cmdFlags.labelPrefix, "labelprefix", "l", "", "Label prefix for ecs tag")
	f.StringVarP(&cmdFlags.instanceName, "instancename", "n", "", "filter by instance name")
	f.IntVarP(&cmdFlags.pageSize, "pagesize", "s", 10, "alicloud api pagesize")
	f.VarP(&cmdFlags.tag, "tag", "t", "filter by ecs instance tag example: cluster=prod (can specify multiple)")
	f.VarP(&cmdFlags.regName, "regname", "", "filter by ecs instance name with regular expression  example: ecs.* (can specify multiple, will use or operator)")
	f.VarP(&cmdFlags.noTagKey, "notagk", "", "filter by ecs instance tag key not contain keyword with regular expression example: acs:autoscaling.* (can specify multiple)")
	f.VarP(&cmdFlags.noTagValue, "notagv", "", "filter by ecs instance tag value not contain keyword with regular expression example: autoScale (can specify multiple)")
}
