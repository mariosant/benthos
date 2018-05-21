![Benthos](icon.png "Benthos")

[![godoc for Jeffail/benthos][1]][2]
[![goreportcard for Jeffail/benthos][3]][4]
[![Build Status][travis-badge]][travis-url]

Benthos is a stream multiplexer service driven by a [config file](config) in
either YAML or JSON format and deploys as a single binary with zero runtime
dependencies.

A variety of [configurable message processors][10] can be chained together for
solving common streaming problems such as content based multiplexing, filtering,
modifying, batching, splitting, (de)compressing, (un)archiving, etc.

A range of optional [buffer][12] strategies are available, allowing you to
select a balance between latency, protection against back pressure and file
based persistence, or nothing at all (direct bridge).

For more details [check out the general documentation][general-docs]. For some
applied examples such as streaming and deduplicating the Twitter firehose to
Kafka [check out the cookbook section][cookbook-docs].

## Supported Protocols

Currently supported input/output targets:

- [Amazon (S3, SQS)][amazons3]
- [Elasticsearch][elasticsearch]
- File
- HTTP(S)
- [Kafka][kafka]
- [MQTT][mqtt]
- [Nanomsg][nanomsg]
- [NATS][nats]
- [NATS Streaming][natsstreaming]
- [NSQ][nsq]
- [RabbitMQ (AMQP 0.91)][rabbitmq]
- [Redis][redis]
- Stdin/Stdout
- [ZMQ4][zmq]

Setting up multiple outputs or inputs is done by choosing a routing strategy
(fan-in, fan-out, round-robin, etc.)

It is possible to enable a REST API to dynamically change inputs and outputs at
runtime, [which you can read about here][11].

For a full and up to date list of all inputs, buffer options, processors, and
outputs [you can find them in the docs][7], or print them from the binary:

```
# Print inputs, buffers and output options
benthos --list-inputs --list-buffers --list-outputs --list-processors | less
```

Mixing multiple part message protocols with single part can be done in different
ways, for more guidance [check out this doc.][5]

## Install

Build with Go:

``` shell
go get github.com/Jeffail/benthos/cmd/benthos
```

Or, pull the docker image:

``` shell
docker pull jeffail/benthos
```

Or, [download from here.](https://github.com/Jeffail/benthos/releases)

## Run

``` shell
benthos -c ./config.yaml
```

Or, with docker:

``` shell
# Send HTTP /POST data to Kafka:
docker run --rm \
	-e "BENTHOS_INPUT=http_server" \
	-e "BENTHOS_OUTPUT=kafka" \
	-e "KAFKA_OUTPUT_BROKER_ADDRESSES=kafka-server:9092" \
	-e "KAFKA_OUTPUT_TOPIC=benthos_topic" \
	-p 4195:4195 \
	jeffail/benthos

# Using your own config file:
docker run --rm -v /path/to/your/config.yaml:/benthos.yaml jeffail/benthos
```

## Streams

Benthos can be run in `--streams` mode which, instead of running a single stream
of inputs to outputs, opens up a [REST HTTP API][streams-api] for creating and
managing multiple streams. Each stream has its own input, buffer, pipeline and
output sections which contains an isolated stream of data with its own lifetime.

## Config

Benthos has inputs, optional processors, an optional buffer, and outputs, which
are all set in a single config file.

Check out the samples in [./config](config), or create a fully populated default
configuration file:

``` shell
benthos --print-yaml > config.yaml
benthos --print-json | jq '.' > config.json
```

If we wanted to pipe Stdin to a ZMQ push socket our YAML config might look like
this:

``` yaml
input:
  type: stdin
output:
  type: zmq4
  zmq4:
    addresses:
      - tcp://*:1234
    socket_type: PUSH
```

There are also configuration sections for logging and metrics, if you print an
example config you will see the available options.

For a list of metrics within Benthos [check out this spec][6].

### Environment Variables

[You can use environment variables][8] to replace fields in your config files.

## ZMQ4 Support

Benthos supports ZMQ4 for both data input and output. To add this you need to
install libzmq4 and use the compile time flag when building Benthos:

``` shell
go install -tags "ZMQ4" ./cmd/...
```

## Vendoring

Benthos uses [dep][dep] for managing dependencies. To get started make sure you
have dep installed:

`go get -u github.com/golang/dep/cmd/dep`

And then run `dep ensure`. You can decrease the size of vendor by only storing
needed files with `dep prune`.

## Docker

There's a multi-stage `Dockerfile` for creating a Benthos docker image which
results in a minimal image from scratch. You can build it with:

``` shell
make docker
```

Then use the image:

``` shell
docker run --rm \
	-v /path/to/your/benthos.yaml:/config.yaml \
	-v /tmp/data:/data \
	-p 4195:4195 \
	benthos -c /config.yaml
```

There are a [few examples here][9] that show you some ways of setting up Benthos
containers using `docker-compose`.

[1]: https://godoc.org/github.com/Jeffail/benthos?status.svg
[2]: https://godoc.org/github.com/Jeffail/benthos
[3]: https://goreportcard.com/badge/github.com/Jeffail/benthos
[4]: https://goreportcard.com/report/Jeffail/benthos
[5]: resources/docs/multipart.md
[6]: resources/docs/metrics.md
[7]: resources/docs/README.md
[8]: resources/docs/config_interpolation.md
[9]: resources/docker/compose_examples
[10]: resources/docs/processors/README.md
[11]: resources/docs/dynamic_inputs_and_outputs.md
[12]: resources/docs/buffers/README.md
[streams-api]: resources/docs/api/streams.md
[general-docs]: resources/docs/README.md#benthos
[cookbook-docs]: resources/docs/cookbook/README.md
[travis-badge]: https://travis-ci.org/Jeffail/benthos.svg?branch=master
[travis-url]: https://travis-ci.org/Jeffail/benthos
[dep]: https://github.com/golang/dep
[amazons3]: https://aws.amazon.com/s3/
[zmq]: http://zeromq.org/
[nanomsg]: http://nanomsg.org/
[rabbitmq]: https://www.rabbitmq.com/
[mqtt]: http://mqtt.org/
[nsq]: http://nsq.io/
[nats]: http://nats.io/
[natsstreaming]: https://nats.io/documentation/streaming/nats-streaming-intro/
[redis]: https://redis.io/
[kafka]: https://kafka.apache.org/
[elasticsearch]: https://www.elastic.co/
