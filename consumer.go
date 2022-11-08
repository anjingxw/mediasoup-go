package mediasoup

import (
	"encoding/json"
	"sync/atomic"

	"github.com/go-logr/logr"
)

// ConsumerOptions define options to create a Consumer.
type ConsumerOptions struct {
	// ProducerId is the id of the Producer to consume.
	ProducerId string `json:"producerId,omitempty"`

	// RtpCapabilities is RTP capabilities of the consuming endpoint.
	RtpCapabilities RtpCapabilities `json:"rtpCapabilities,omitempty"`

	// Paused define whether the Consumer must start in paused mode. Default false.
	//
	// When creating a video Consumer, it's recommended to set paused to true,
	// then transmit the Consumer parameters to the consuming endpoint and, once
	// the consuming endpoint has created its local side Consumer, unpause the
	// server side Consumer using the resume() method. This is an optimization
	// to make it possible for the consuming endpoint to render the video as far
	// as possible. If the server side Consumer was created with paused false,
	// mediasoup will immediately request a key frame to the remote Producer and
	// suych a key frame may reach the consuming endpoint even before it's ready
	// to consume it, generating “black” video until the device requests a keyframe
	// by itself.
	Paused bool `json:"paused,omitempty"`

	// Mid is the MID for the Consumer. If not specified, a sequentially growing
	// number will be assigned.
	Mid string `json:"mid,omitempty"`

	//
	// PreferredLayers define preferred spatial and temporal layer for simulcast or
	// SVC media sources. If unset, the highest ones are selected.
	PreferredLayers *ConsumerLayers `json:"preferredLayers,omitempty"`

	// IgnoreDtx define whether this Consumer should ignore DTX packets (only valid for
	// Opus codec). If set, DTX packets are not forwarded to the remote Consumer.
	IgnoreDtx bool `json:"ignoreDtx,omitempty"`

	// Pipe define whether this Consumer should consume all RTP streams generated by the
	// Producer.
	Pipe bool `json:"pipe,omitempty"`

	// AppData is custom application data.
	AppData interface{} `json:"appData,omitempty"`

	Ssrc uint32 `json:"ssrc,omitempty"`
}

// ConsumerTraceEventType is valid types for "trace" event.
type ConsumerTraceEventType string

const (
	ConsumerTraceEventType_Rtp      ConsumerTraceEventType = "rtp"
	ConsumerTraceEventType_Keyframe ConsumerTraceEventType = "keyframe"
	ConsumerTraceEventType_Nack     ConsumerTraceEventType = "nack"
	ConsumerTraceEventType_Pli      ConsumerTraceEventType = "pli"
	ConsumerTraceEventType_Fir      ConsumerTraceEventType = "fir"
)

// ConsumerTraceEventData is "trace" event data.
type ConsumerTraceEventData struct {
	// Type is trace type.
	Type ConsumerTraceEventType `json:"type,omitempty"`

	// timestamp is event timestamp.
	Timestamp int64 `json:"timestamp,omitempty"`

	// Direction is event direction, "in" | "out".
	Direction string `json:"direction,omitempty"`

	// Info is per type information.
	Info H `json:"info,omitempty"`
}

// ConsumerScore define "score" event data
type ConsumerScore struct {
	// Score of the RTP stream of the consumer.
	Score uint16 `json:"score"`

	// Score of the currently selected RTP stream of the producer.
	ProducerScore uint16 `json:"producerScore"`

	// ProducerScores is he scores of all RTP streams in the producer ordered
	// by encoding (just useful when the producer uses simulcast).
	ProducerScores []uint16 `json:"producerScores,omitempty"`
}

type ConsumerLayers struct {
	// SpatialLayer is the spatial layer index (from 0 to N).
	SpatialLayer uint8 `json:"spatialLayer"`

	// TemporalLayer is the temporal layer index (from 0 to N).
	TemporalLayer uint8 `json:"temporalLayer"`
}

// ConsumerStat include two entries: the statistics of the RTP stream in the consumer (type: "outbound-rtp")
// and the statistics of the associated RTP stream in the producer (type: "inbound-rtp").
type ConsumerStat struct {
	// Common to all RtpStreams
	Type                 string  `json:"type,omitempty"`
	Timestamp            int64   `json:"timestamp,omitempty"`
	Ssrc                 uint32  `json:"ssrc,omitempty"`
	RtxSsrc              uint32  `json:"rtxSsrc,omitempty"`
	Rid                  string  `json:"rid,omitempty"`
	Kind                 string  `json:"kind,omitempty"`
	MimeType             string  `json:"mimeType,omitempty"`
	PacketsLost          uint32  `json:"packetsLost,omitempty"`
	FractionLost         uint32  `json:"fractionLost,omitempty"`
	PacketsDiscarded     uint32  `json:"packetsDiscarded,omitempty"`
	PacketsRetransmitted uint32  `json:"packetsRetransmitted,omitempty"`
	PacketsRepaired      uint32  `json:"packetsRepaired,omitempty"`
	NackCount            uint32  `json:"nackCount,omitempty"`
	NackPacketCount      uint32  `json:"nackPacketCount,omitempty"`
	PliCount             uint32  `json:"pliCount,omitempty"`
	FirCount             uint32  `json:"firCount,omitempty"`
	Score                uint32  `json:"score,omitempty"`
	PacketCount          int64   `json:"packetCount,omitempty"`
	ByteCount            int64   `json:"byteCount,omitempty"`
	Bitrate              uint32  `json:"bitrate,omitempty"`
	RoundTripTime        float32 `json:"roundTripTime,omitempty"`
	RtxPacketsDiscarded  uint32  `json:"rtxPacketsDiscarded,omitempty"`
}

// ProducerType define Consumer type.
type ConsumerType string

const (
	ConsumerType_Simple    ConsumerType = "simple"
	ConsumerType_Simulcast ConsumerType = "simulcast"
	ConsumerType_Svc       ConsumerType = "svc"
	ConsumerType_Pipe      ConsumerType = "pipe"
)

type consumerParams struct {
	// internal uses routerId, transportId, consumerId, producerId
	internal        internalData
	data            consumerData
	channel         *Channel
	payloadChannel  *PayloadChannel
	appData         interface{}
	paused          bool
	producerPaused  bool
	score           *ConsumerScore
	preferredLayers *ConsumerLayers
}

type consumerData struct {
	ProducerId    string        `json:"producerId,omitempty"`
	Kind          MediaKind     `json:"kind,omitempty"`
	Type          ConsumerType  `json:"type,omitempty"`
	RtpParameters RtpParameters `json:"rtpParameters,omitempty"`
}

// Consumer represents an audio or video source being forwarded from a mediasoup router to an
// endpoint. It's created on top of a transport that defines how the media packets are carried.
//
//   - @emits transportclose
//   - @emits producerclose
//   - @emits producerpause
//   - @emits producerresume
//   - @emits score - (score *ConsumerScore)
//   - @emits layerschange - (layers *ConsumerLayers | nil)
//   - @emits rtp - (packet []byte)
//   - @emits trace - (trace *ConsumerTraceEventData)
//   - @emits @close
//   - @emits @producerclose
type Consumer struct {
	IEventEmitter
	logger           logr.Logger
	internal         internalData
	data             consumerData
	channel          *Channel
	payloadChannel   *PayloadChannel
	appData          interface{}
	paused           bool
	closed           uint32
	producerPaused   bool
	priority         uint32
	score            *ConsumerScore
	preferredLayers  *ConsumerLayers
	currentLayers    *ConsumerLayers // Current video layers (just for video with simulcast or SVC).
	observer         IEventEmitter
	onClose          func()
	onProducerClose  func()
	onTransportClose func()
	onPause          func()
	onResume         func()
	onProducerPause  func()
	onProducerResume func()
	onScore          func(*ConsumerScore)
	onLayersChange   func(*ConsumerLayers)
	onTrace          func(*ConsumerTraceEventData)
	onRtp            func([]byte)
}

func newConsumer(params consumerParams) *Consumer {
	logger := NewLogger("Consumer")

	logger.V(1).Info("constructor()", "internal", params.internal)

	score := params.score

	if score == nil {
		score = &ConsumerScore{
			Score:          10,
			ProducerScore:  10,
			ProducerScores: []uint16{},
		}
	}

	consumer := &Consumer{
		IEventEmitter:   NewEventEmitter(),
		logger:          logger,
		internal:        params.internal,
		data:            params.data,
		channel:         params.channel,
		payloadChannel:  params.payloadChannel,
		appData:         params.appData,
		paused:          params.paused,
		producerPaused:  params.producerPaused,
		priority:        1,
		score:           score,
		preferredLayers: params.preferredLayers,
		observer:        NewEventEmitter(),
	}

	consumer.handleWorkerNotifications()

	return consumer
}

// Id returns consumer id
func (consumer *Consumer) Id() string {
	return consumer.internal.ConsumerId
}

// ConsumerId returns associated Consumer id.
func (consumer *Consumer) ConsumerId() string {
	return consumer.internal.ConsumerId
}

// ProducerId returns associated Producer id.
func (consumer *Consumer) ProducerId() string {
	return consumer.data.ProducerId
}

// Closed returns whether the Consumer is closed.
func (consumer *Consumer) Closed() bool {
	return atomic.LoadUint32(&consumer.closed) > 0
}

// Kind returns media kind.
func (consumer *Consumer) Kind() MediaKind {
	return consumer.data.Kind
}

// RtpParameters returns RTP parameters.
func (consumer *Consumer) RtpParameters() RtpParameters {
	return consumer.data.RtpParameters
}

// Type returns consumer type.
func (consumer *Consumer) Type() ConsumerType {
	return consumer.data.Type
}

// Paused returns whether the Consumer is paused.
func (consumer *Consumer) Paused() bool {
	return consumer.paused
}

// ProducerPaused returns whether the associate Producer is paused.
func (consumer *Consumer) ProducerPaused() bool {
	return consumer.producerPaused
}

// Priority returns current priority.
func (consumer *Consumer) Priority() uint32 {
	return consumer.priority
}

// Score returns consumer score with consumer and consumer keys.
func (consumer *Consumer) Score() *ConsumerScore {
	return consumer.score
}

// PreferredLayers returns preferred video layers.
func (consumer *Consumer) PreferredLayers() *ConsumerLayers {
	return consumer.preferredLayers
}

// CurrentLayers returns current video layers.
func (consumer *Consumer) CurrentLayers() *ConsumerLayers {
	return consumer.currentLayers
}

// AppData returns app custom data.
func (consumer *Consumer) AppData() interface{} {
	return consumer.appData
}

// Deprecated
//
//   - @emits close
//   - @emits pause
//   - @emits resume
//   - @emits score - (score *ConsumerScore)
//   - @emits layerschange - (layers *ConsumerLayers | nil)
//   - @emits trace - (trace *ConsumerTraceEventData)
func (consumer *Consumer) Observer() IEventEmitter {
	return consumer.observer
}

// Close the Consumer.
func (consumer *Consumer) Close() (err error) {
	if atomic.CompareAndSwapUint32(&consumer.closed, 0, 1) {
		consumer.logger.V(1).Info("close()")

		// Remove notification subscriptions.
		consumer.channel.Unsubscribe(consumer.internal.ConsumerId)
		consumer.payloadChannel.Unsubscribe(consumer.internal.ConsumerId)

		reqData := H{"consumerId": consumer.internal.ConsumerId}

		response := consumer.channel.Request("transport.closeConsumer", consumer.internal, reqData)
		if err = response.Err(); err != nil {
			consumer.logger.Error(err, "consumer close failed")
		}

		consumer.Emit("@close")
		consumer.RemoveAllListeners()

		consumer.close()
	}
	return
}

// close send "close" event.
func (consumer *Consumer) close() {
	// Emit observer event.
	consumer.observer.SafeEmit("close")
	consumer.observer.RemoveAllListeners()

	if handler := consumer.onClose; handler != nil {
		handler()
	}
}

// transportClosed is called when transport was closed.
func (consumer *Consumer) transportClosed() {
	if atomic.CompareAndSwapUint32(&consumer.closed, 0, 1) {
		consumer.logger.V(1).Info("transportClosed()")

		// Remove notification subscriptions.
		consumer.channel.Unsubscribe(consumer.internal.ConsumerId)
		consumer.payloadChannel.Unsubscribe(consumer.internal.ConsumerId)

		consumer.SafeEmit("transportclose")
		consumer.RemoveAllListeners()

		if handler := consumer.onTransportClose; handler != nil {
			handler()
		}

		consumer.close()
	}
}

// Dump Consumer.
func (consumer *Consumer) Dump() (dump *ConsumerDump, err error) {
	consumer.logger.V(1).Info("dump()")

	resp := consumer.channel.Request("consumer.dump", consumer.internal)
	err = resp.Unmarshal(&dump)

	return
}

// GetStats returns Consumer stats.
func (consumer *Consumer) GetStats() (stats []*ConsumerStat, err error) {
	consumer.logger.V(1).Info("getStats()")

	resp := consumer.channel.Request("consumer.getStats", consumer.internal)
	err = resp.Unmarshal(&stats)

	return
}

// Pause the Consumer.
func (consumer *Consumer) Pause() (err error) {
	consumer.logger.V(1).Info("pause()")

	wasPaused := consumer.paused || consumer.producerPaused

	response := consumer.channel.Request("consumer.pause", consumer.internal)

	if err = response.Err(); err != nil {
		return
	}

	consumer.paused = true

	// Emit observer event.
	if !wasPaused {
		consumer.observer.SafeEmit("pause")

		if handler := consumer.onPause; handler != nil {
			handler()
		}
	}

	return
}

// Resume the Consumer.
func (consumer *Consumer) Resume() (err error) {
	consumer.logger.V(1).Info("resume()")

	wasPaused := consumer.paused || consumer.producerPaused

	response := consumer.channel.Request("consumer.resume", consumer.internal)

	if err = response.Err(); err != nil {
		return
	}

	consumer.paused = false

	// Emit observer event.
	if wasPaused && !consumer.producerPaused {
		consumer.observer.SafeEmit("resume")

		if handler := consumer.onResume; handler != nil {
			handler()
		}
	}

	return
}

// SetPreferredLayers set preferred video layers.
func (consumer *Consumer) SetPreferredLayers(layers ConsumerLayers) (err error) {
	consumer.logger.V(1).Info("setPreferredLayers()")

	response := consumer.channel.Request("consumer.setPreferredLayers", consumer.internal, layers)
	err = response.Unmarshal(&consumer.preferredLayers)

	return
}

// SetPriority set priority.
func (consumer *Consumer) SetPriority(priority uint32) (err error) {
	consumer.logger.V(1).Info("setPriority()")

	response := consumer.channel.Request("consumer.setPriority", consumer.internal, H{"priority": priority})

	var result struct {
		Priority uint32
	}
	if err = response.Unmarshal(&result); err != nil {
		return
	}

	consumer.priority = result.Priority

	return
}

// UnsetPriority unset priority.
func (consumer *Consumer) UnsetPriority() (err error) {
	consumer.logger.V(1).Info("unsetPriority()")

	return consumer.SetPriority(1)
}

// RequestKeyFrame request a key frame to the Producer.
func (consumer *Consumer) RequestKeyFrame() error {
	consumer.logger.V(1).Info("requestKeyFrame()")

	response := consumer.channel.Request("consumer.requestKeyFrame", consumer.internal)

	return response.Err()
}

// EnableTraceEvent eenable "trace" event.
func (consumer *Consumer) EnableTraceEvent(types ...ConsumerTraceEventType) error {
	consumer.logger.V(1).Info("enableTraceEvent()")

	if types == nil {
		types = []ConsumerTraceEventType{}
	}

	response := consumer.channel.Request("consumer.enableTraceEvent", consumer.internal, H{"types": types})

	return response.Err()
}

// OnClose set handler on "close" event
func (consumer *Consumer) OnClose(handler func()) {
	consumer.onClose = handler
}

// OnProducerClose set handler on "producerclose" event
func (consumer *Consumer) OnProducerClose(handler func()) {
	consumer.onProducerClose = handler
}

// OnTransportClose set handler on "transportclose" event
func (consumer *Consumer) OnTransportClose(handler func()) {
	consumer.onTransportClose = handler
}

// OnPause set handler on "pause" event
func (consumer *Consumer) OnPause(handler func()) {
	consumer.onPause = handler
}

// OnResume set handler on "resume" event
func (consumer *Consumer) OnResume(handler func()) {
	consumer.onResume = handler
}

// OnProducerPause set handler on "producerpause" event
func (consumer *Consumer) OnProducerPause(handler func()) {
	consumer.onProducerPause = handler
}

// OnProducerResume set handler on "producerresume" event
func (consumer *Consumer) OnProducerResume(handler func()) {
	consumer.onProducerResume = handler
}

// OnScore set handler on "score" event
func (consumer *Consumer) OnScore(handler func(score *ConsumerScore)) {
	consumer.onScore = handler
}

// OnLayersChange set handler on "layerschange" event
func (consumer *Consumer) OnLayersChange(handler func(layers *ConsumerLayers)) {
	consumer.onLayersChange = handler
}

// OnTrace set handler on "trace" event
func (consumer *Consumer) OnTrace(handler func(trace *ConsumerTraceEventData)) {
	consumer.onTrace = handler
}

// OnRtp set handler on "rtp" event
func (consumer *Consumer) OnRtp(handler func(data []byte)) {
	consumer.onRtp = handler
}

func (consumer *Consumer) handleWorkerNotifications() {
	logger := consumer.logger

	consumer.channel.Subscribe(consumer.Id(), func(event string, data []byte) {
		switch event {
		case "producerclose":
			if atomic.CompareAndSwapUint32(&consumer.closed, 0, 1) {
				consumer.channel.Unsubscribe(consumer.internal.ConsumerId)
				consumer.payloadChannel.Unsubscribe(consumer.internal.ConsumerId)

				consumer.Emit("@producerclose")
				consumer.SafeEmit("producerclose")
				consumer.RemoveAllListeners()

				if handler := consumer.onProducerClose; handler != nil {
					handler()
				}

				consumer.close()
			}

		case "producerpause":
			if consumer.producerPaused {
				break
			}

			wasPaused := consumer.paused || consumer.producerPaused

			consumer.producerPaused = true

			consumer.SafeEmit("producerpause")

			if handler := consumer.onProducerPause; handler != nil {
				handler()
			}

			if !wasPaused {
				// Emit observer event.
				consumer.observer.SafeEmit("pause")

				if handler := consumer.onPause; handler != nil {
					handler()
				}
			}

		case "producerresume":
			if !consumer.producerPaused {
				break
			}

			wasPaused := consumer.paused || consumer.producerPaused

			consumer.producerPaused = false

			consumer.SafeEmit("producerresume")

			if handler := consumer.onProducerResume; handler != nil {
				handler()
			}

			if wasPaused && !consumer.paused {
				// Emit observer event.
				consumer.observer.SafeEmit("resume")

				if handler := consumer.onResume; handler != nil {
					handler()
				}
			}

		case "score":
			var score *ConsumerScore

			if err := json.Unmarshal([]byte(data), &score); err != nil {
				logger.Error(err, "failed to unmarshal score", "data", json.RawMessage(data))
				return
			}

			consumer.score = score

			consumer.SafeEmit("score", score)

			// Emit observer event.
			consumer.observer.SafeEmit("score", &score)

			if handler := consumer.onScore; handler != nil {
				handler(score)
			}

		case "layerschange":
			var layers *ConsumerLayers

			if err := json.Unmarshal([]byte(data), &layers); err != nil {
				logger.Error(err, "failed to unmarshal layers", "data", json.RawMessage(data))
				return
			}

			consumer.currentLayers = layers

			consumer.SafeEmit("layerschange", layers)

			// Emit observer event.
			consumer.observer.SafeEmit("layerschange", layers)

			if handler := consumer.onLayersChange; handler != nil {
				handler(layers)
			}

		case "trace":
			var trace *ConsumerTraceEventData

			if err := json.Unmarshal([]byte(data), &trace); err != nil {
				logger.Error(err, "failed to unmarshal trace", "data", json.RawMessage(data))
				return
			}

			consumer.SafeEmit("trace", trace)

			// Emit observer event.
			consumer.observer.SafeEmit("trace", trace)

			if handler := consumer.onTrace; handler != nil {
				handler(trace)
			}

		default:
			consumer.logger.Error(nil, "ignoring unknown event in channel listener", "event", event)
		}
	})

	consumer.payloadChannel.Subscribe(consumer.Id(), func(event string, data, payload []byte) {
		switch event {
		case "rtp":
			if consumer.Closed() {
				return
			}
			consumer.SafeEmit("rtp", payload)

			if handler := consumer.onRtp; handler != nil {
				handler(payload)
			}

		default:
			consumer.logger.Error(nil, "ignoring unknown event in payload channel listener", "event", event)
		}
	})
}
