package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Cepave/common/model"
	stats "github.com/montanaflynn/stats"
)

type ParamToAgent struct {
	Metric      string      `json:"metric"`
	Endpoint    string      `json:"endpoint"`
	Value       interface{} `json:"value"`
	CounterType string      `json:"counterType"`
	Tags        string      `json:"tags"`
	Timestamp   int64       `json:"timestamp"`
	Step        int64       `json:"step"`
}

func (p ParamToAgent) String() string {
	return fmt.Sprintf(
		" {metric: %v, endpoint: %v, value: %v, counterType:%v, tags:%v, timestamp:%d, step:%d}",
		p.Metric,
		p.Endpoint,
		p.Value,
		p.CounterType,
		p.Tags,
		p.Timestamp,
		p.Step,
	)
}

func parseFpingRow(row string) []string {
	return strings.FieldsFunc(row, func(r rune) bool {
		switch r {
		case ' ', '\n', ':', '/', '%', '=', ',':
			return true
		}
		return false
	})
}

type nqmEndpointData struct {
	Id         string
	IspId      string
	ProvinceId string
	CityId     string
	NameTagId  string
}

func agentToNqmEndpointData(s *model.NqmAgent) *nqmEndpointData {
	return &nqmEndpointData{
		Id:         strconv.Itoa(s.Id),
		IspId:      strconv.Itoa(int(s.IspId)),
		ProvinceId: strconv.Itoa(int(s.ProvinceId)),
		CityId:     strconv.Itoa(int(s.CityId)),
		NameTagId:  strconv.Itoa(-1),
	}
}

func targetToNqmEndpointData(s *model.NqmTarget) *nqmEndpointData {
	return &nqmEndpointData{
		Id:         strconv.Itoa(s.Id),
		IspId:      strconv.Itoa(int(s.IspId)),
		ProvinceId: strconv.Itoa(int(s.ProvinceId)),
		CityId:     strconv.Itoa(int(s.CityId)),
		NameTagId:  strconv.Itoa(-1),
	}
}

func marshalFpingRowIntoJSON(row []string, target model.NqmTarget, agentPtr *model.NqmAgent) []ParamToAgent {
	var params []ParamToAgent

	nqmStat := nqmFpingStat(row)
	params = append(params, marshalJSON(target, agentPtr, "packets-sent", nqmStat["pkttransmit"]))
	params = append(params, marshalJSON(target, agentPtr, "packets-received", nqmStat["pktreceive"]))
	params = append(params, marshalJSON(target, agentPtr, "transmission-time", nqmStat["rttavg"]))

	t := targetToNqmEndpointData(&target)
	a := agentToNqmEndpointData(agentPtr)
	nqmDataGram := nqmTagsAssembler(t, a, nqmStat)
	params = append(params, nqmMarshalJSON(nqmDataGram, "nqm-metrics"))
	return params
}

func nqmParseFpingRow(row []string) map[string]string {
	/*
		www.yahoo.com  : xmt/rcv/%loss = 100/99/1%, min/avg/max = 5.42/10.9/35.9
		 0                1   2   3       4  5  6                  10   11  12
	*/
	nqmDataMap := map[string]string{}
	nqmDataMap["rttmin"] = row[10]
	nqmDataMap["rttmax"] = row[12]
	nqmDataMap["rttavg"] = row[11]
	nqmDataMap["rttmdev"] = "-1"
	nqmDataMap["rttmedian"] = "-1"
	nqmDataMap["pkttransmit"] = row[4]
	nqmDataMap["pktreceive"] = row[5]
	return nqmDataMap
}

func nqmFpingStat(row []string) map[string]string {
	/*
		    assume fping command looks like:
		        fping -p 20 -i 10 -C 5 -a www.google.com www.yahoo.com
		    input argument row looks like:
				www.yahoo.com  6.72 29.08 8.55 7.40 - 6.26
				0                1   2     3     4  5   6   ....  n
	*/
	var data []float64

	for i := 1; i < len(row); i++ {
		if row[i] != "-" {
			rtt, err := strconv.ParseFloat(row[i], 64)
			if err != nil {
				log.Println("error occured:", err)
			} else {
				data = append(data, rtt)
			}
		}
	}

	pktxmt := len(row) - 1
	pktrcv := len(data)
	var d stats.Float64Data = data
	median, _ := d.Median()
	max, _ := d.Max()
	min, _ := d.Min()
	mean, _ := d.Mean()
	dev, _ := d.StandardDeviation()

	nqmDataMap := map[string]string{
		"rttmin":    "-1",
		"rttmax":    "-1",
		"rttavg":    "-1",
		"rttmdev":   "-1",
		"rttmedian": "-1",
	}
	if len(data) > 0 {
		nqmDataMap["rttmin"] = strconv.FormatFloat(min, 'f', 2, 64)
		nqmDataMap["rttmax"] = strconv.FormatFloat(max, 'f', 2, 64)
		nqmDataMap["rttavg"] = strconv.FormatFloat(mean, 'f', 2, 64)
		nqmDataMap["rttmdev"] = strconv.FormatFloat(dev, 'f', 2, 64)
		nqmDataMap["rttmedian"] = strconv.FormatFloat(median, 'f', 2, 64)
	}
	nqmDataMap["pkttransmit"] = strconv.Itoa(pktxmt)
	nqmDataMap["pktreceive"] = strconv.Itoa(pktrcv)
	return nqmDataMap
}

func nqmTagsAssembler(target *nqmEndpointData, agent *nqmEndpointData, nqmDataMap map[string]string) string {
	return "agent-id=" + agent.Id +
		",agent-isp-id=" + agent.IspId +
		",agent-province-id=" + agent.ProvinceId +
		",agent-city-id=" + agent.CityId +
		",agent-name-tag-id=" + agent.NameTagId +
		",target-id=" + target.Id +
		",target-isp-id=" + target.IspId +
		",target-province-id=" + target.ProvinceId +
		",target-city-id=" + target.CityId +
		",target-name-tag-id=" + target.NameTagId +
		",rttmin=" + nqmDataMap["rttmin"] +
		",rttmax=" + nqmDataMap["rttmax"] +
		",rttavg=" + nqmDataMap["rttavg"] +
		",rttmdev=" + nqmDataMap["rttmdev"] +
		",rttmedian=" + nqmDataMap["rttmedian"] +
		",pkttransmit=" + nqmDataMap["pkttransmit"] +
		",pktreceive=" + nqmDataMap["pktreceive"]
}

func nqmMarshalJSON(nqmDataGram string, metric string) ParamToAgent {
	data := ParamToAgent{}
	data.Tags = nqmDataGram
	data.Metric = metric
	data.Timestamp = time.Now().Unix()
	data.Endpoint = GetGeneralConfig().Hostname
	data.Value = "0"
	data.CounterType = "GAUGE"
	data.Step = int64(60)
	return data
}

/**
 * value could be:
 *     Packet Loss - int
 *     Transmission Time - float64
 */

func marshalJSON(target model.NqmTarget, agent *model.NqmAgent, metric string, value interface{}) ParamToAgent {
	endpoint := GetGeneralConfig().Hostname
	counterType := "GAUGE"
	tags := "nqm-agent-isp=" + agent.IspName +
		",nqm-agent-province=" + agent.ProvinceName +
		",nqm-agent-city=" + agent.CityName +
		",target-ip=" + target.Host +
		",target-isp=" + target.IspName +
		",target-province=" + target.ProvinceName +
		",target-city=" + target.CityName +
		",target-name-tag=" + target.NameTag
	timestamp := time.Now().Unix()
	step := int64(60)
	return ParamToAgent{metric, endpoint, value, counterType, tags, timestamp, step}
}

func MarshalIntoParameters(rawData []string, targetList []model.NqmTarget, agentPtr *model.NqmAgent) []ParamToAgent {
	var params []ParamToAgent
	for rowNum, row := range rawData {
		parsedRow := parseFpingRow(row)
		target := targetList[rowNum]
		params = append(params, marshalFpingRowIntoJSON(parsedRow, target, agentPtr)...)
	}
	return params
}
