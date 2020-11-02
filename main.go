package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

type ombResults struct {
	ConsumeRate         []float64 `json:"consumeRate"`
	EndToEndLatencyAvg  []float64 `json:"endToEndLatencyAvg"`
	PublishLatency99pct []float64 `json:"publishLatency99pct"`
	PublishRate         []float64 `json:"publishRate"`
}

type sample struct {
	labels labels.Labels
	t      int64
	v      float64
}

func labelsToLabelsProto(labels labels.Labels, buf []prompb.Label) []prompb.Label {
	result := buf[:0]
	if cap(buf) < len(labels) {
		result = make([]prompb.Label, 0, len(labels))
	}
	for _, l := range labels {
		result = append(result, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}

func addSamples(data []float64, metricName string, timeseries *prompb.TimeSeries, additionalLabels labels.Labels) {
	for i := range data {
		sample := sample{
			labels: append(labels.Labels{{Name: "__name__", Value: metricName}}, additionalLabels...),
			t:      timestamp.FromTime(time.Now().Add(time.Second * time.Duration(-10*(len(data)-i)))),
			v:      data[i],
		}

		timeseries.Labels = labelsToLabelsProto(sample.labels, timeseries.Labels)
		timeseries.Samples[i].Timestamp = sample.t
		timeseries.Samples[i].Value = sample.v
	}
}

func main() {
	thanosPtr := flag.String("thanos", "", "Thanos URL.")
	jsonPtr := flag.String("results", "", "OMB results json path.")
	labelsPtr := flag.String("labels", "", "Additional label:value pairs (separated by comma).")
	insecurePtr := flag.Bool("insecure", false, "TLS insecure skip verify.")
	tokenPtr := flag.String("token", "", "Bearer token.")

	flag.Parse()

	thanosURL := *thanosPtr
	if thanosURL == "" {
		thanosURL = os.Getenv("THANOS_RECEIVER_URL")
	}

	token := *tokenPtr
	if token == "" {
		token = os.Getenv("THANOS_BEARER_TOKEN")
	}

	labelsArr := strings.Split(*labelsPtr, ",")

	additionalLabels := make(labels.Labels, len(labelsArr))

	for i := range labelsArr {
		pair := strings.Split(labelsArr[i], ":")
		additionalLabels[i] = labels.Label{Name: pair[0], Value: pair[1]}
	}

	file, _ := ioutil.ReadFile(*jsonPtr)

	results := ombResults{}

	err := json.Unmarshal([]byte(file), &results)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	serverURL, err := url.Parse(thanosURL + "/api/v1/receive")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	conf := &remote.ClientConfig{
		URL:     &config.URL{URL: serverURL},
		Timeout: model.Duration(time.Second),
		HTTPClientConfig: config.HTTPClientConfig{
			BearerToken: config.Secret(token),
			TLSConfig: config.TLSConfig{
				InsecureSkipVerify: *insecurePtr,
			},
		},
	}

	c, err := remote.NewWriteClient("load-test", conf)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	timeseries := make([]prompb.TimeSeries, 4)
	for i := range timeseries {
		timeseries[i].Samples = make([]prompb.Sample, len(results.ConsumeRate))
	}

	addSamples(results.ConsumeRate, "omb_results_consume_rate", &timeseries[0], additionalLabels)
	addSamples(results.EndToEndLatencyAvg, "omb_results_end_to_end_latency_avg", &timeseries[1], additionalLabels)
	addSamples(results.PublishLatency99pct, "omb_results_publish_latency_99pct", &timeseries[2], additionalLabels)
	addSamples(results.PublishRate, "omb_results_publish_rate", &timeseries[3], additionalLabels)

	var buf []byte

	req := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	compressed := snappy.Encode(buf, data)

	buf = compressed

	err = c.Store(context.Background(), buf)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
