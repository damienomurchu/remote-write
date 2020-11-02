# Remote-write

Tool for pushing OMB test results to thanos-receiver.

It expects json file with results generated by [mk-performce-tests](https://gitlab.cee.redhat.com/mk-bin-packing/mk-performance-tests).

It only sends these results:

- `consumeRate`
- `endToEndLatencyAvg`
- `publishLatency99pct`
- `publishRat`

Tool assumes that it is executed right after OMB test finishes and that samples are taken every 10s. It will add timestamps to samples accordingly.

To build run:

```bash
mkdir -p ~/go/src/github.com/jhellar
cd ~/go/src/github.com/jhellar
git clone https://github.com/jhellar/remote-write.git
cd remote-write
make
```

Usage info:

```text
./remote-write -h
Usage of ./remote-write:
  -insecure
    TLS insecure skip verify.
  -labels string
    Additional label:value pairs (separated by comma).
  -results string
    OMB results json path.
  -thanos string
    Thanos URL.
  -token string
    Bearer token.
```

Example usage:

```bash
./remote-write -thanos="http://thanos-receiver-route..." -results="./results.json" -labels="cluster_id:perf_test_cluster_a" -insecure -token="..."
```
