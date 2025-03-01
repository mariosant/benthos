package stream

import (
	"bytes"
	"net/http"
	"runtime/pprof"
	"time"

	"github.com/benthosdev/benthos/v4/internal/bundle"
	"github.com/benthosdev/benthos/v4/internal/component"
	ibuffer "github.com/benthosdev/benthos/v4/internal/component/buffer"
	iinput "github.com/benthosdev/benthos/v4/internal/component/input"
	ioutput "github.com/benthosdev/benthos/v4/internal/component/output"
	"github.com/benthosdev/benthos/v4/internal/message"
	"github.com/benthosdev/benthos/v4/internal/pipeline"
)

//------------------------------------------------------------------------------

// Type creates and manages the lifetime of a Benthos stream.
type Type struct {
	conf Config

	inputLayer    iinput.Streamed
	bufferLayer   ibuffer.Streamed
	pipelineLayer pipeline.Type
	outputLayer   ioutput.Streamed

	manager bundle.NewManagement

	onClose func()
}

// New creates a new stream.Type.
func New(conf Config, mgr bundle.NewManagement, opts ...func(*Type)) (*Type, error) {
	t := &Type{
		conf:    conf,
		manager: mgr,
		onClose: func() {},
	}
	for _, opt := range opts {
		opt(t)
	}
	if err := t.start(); err != nil {
		return nil, err
	}

	healthCheck := func(w http.ResponseWriter, r *http.Request) {
		inputConnected := t.inputLayer.Connected()
		outputConnected := t.outputLayer.Connected()

		if inputConnected && outputConnected {
			_, _ = w.Write([]byte("OK"))
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		if !inputConnected {
			_, _ = w.Write([]byte("input not connected\n"))
		}
		if !outputConnected {
			_, _ = w.Write([]byte("output not connected\n"))
		}
	}
	t.manager.RegisterEndpoint(
		"/ready",
		"Returns 200 OK if all inputs and outputs are connected, otherwise a 503 is returned.",
		healthCheck,
	)
	return t, nil
}

//------------------------------------------------------------------------------

// OptOnClose sets a closure to be called when the stream closes.
func OptOnClose(onClose func()) func(*Type) {
	return func(t *Type) {
		t.onClose = onClose
	}
}

//------------------------------------------------------------------------------

// IsReady returns a boolean indicating whether both the input and output layers
// of the stream are connected.
func (t *Type) IsReady() bool {
	return t.inputLayer.Connected() && t.outputLayer.Connected()
}

func (t *Type) start() (err error) {
	// Constructors
	iMgr := t.manager.IntoPath("input")
	if t.inputLayer, err = iMgr.NewInput(t.conf.Input); err != nil {
		return
	}
	if t.conf.Buffer.Type != "none" {
		bMgr := t.manager.IntoPath("buffer")
		if t.bufferLayer, err = bMgr.NewBuffer(t.conf.Buffer); err != nil {
			return
		}
	}
	if tLen := len(t.conf.Pipeline.Processors); tLen > 0 {
		pMgr := t.manager.IntoPath("pipeline")
		if t.pipelineLayer, err = pipeline.New(t.conf.Pipeline, pMgr); err != nil {
			return
		}
	}
	oMgr := t.manager.IntoPath("output")
	if t.outputLayer, err = oMgr.NewOutput(t.conf.Output); err != nil {
		return
	}

	// Start chaining components
	var nextTranChan <-chan message.Transaction

	nextTranChan = t.inputLayer.TransactionChan()
	if t.bufferLayer != nil {
		if err = t.bufferLayer.Consume(nextTranChan); err != nil {
			return
		}
		nextTranChan = t.bufferLayer.TransactionChan()
	}
	if t.pipelineLayer != nil {
		if err = t.pipelineLayer.Consume(nextTranChan); err != nil {
			return
		}
		nextTranChan = t.pipelineLayer.TransactionChan()
	}
	if err = t.outputLayer.Consume(nextTranChan); err != nil {
		return
	}

	go func(out ioutput.Streamed) {
		for {
			if err := out.WaitForClose(time.Second); err == nil {
				t.onClose()
				return
			}
		}
	}(t.outputLayer)

	return nil
}

// StopGracefully attempts to close the stream in the most graceful way by only
// closing the input layer and waiting for all other layers to terminate by
// proxy. This should guarantee that all in-flight and buffered data is resolved
// before shutting down.
func (t *Type) StopGracefully(timeout time.Duration) (err error) {
	t.inputLayer.CloseAsync()
	started := time.Now()
	if err = t.inputLayer.WaitForClose(timeout); err != nil {
		return
	}

	var remaining time.Duration

	// If we have a buffer then wait right here. We want to try and allow the
	// buffer to empty out before prompting the other layers to shut down.
	if t.bufferLayer != nil {
		t.bufferLayer.StopConsuming()
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.bufferLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	// After this point we can start closing the remaining components.
	if t.pipelineLayer != nil {
		t.pipelineLayer.CloseAsync()
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.pipelineLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	t.outputLayer.CloseAsync()
	remaining = timeout - time.Since(started)
	if remaining < 0 {
		return component.ErrTimeout
	}
	if err = t.outputLayer.WaitForClose(remaining); err != nil {
		return
	}

	return nil
}

// StopOrdered attempts to close all components of the stream in the order of
// positions within the stream, this allows data to flush all the way through
// the pipeline under certain circumstances but is less graceful than
// stopGracefully, which should be attempted first.
func (t *Type) StopOrdered(timeout time.Duration) (err error) {
	t.inputLayer.CloseAsync()
	started := time.Now()
	if err = t.inputLayer.WaitForClose(timeout); err != nil {
		return
	}

	var remaining time.Duration

	if t.bufferLayer != nil {
		t.bufferLayer.CloseAsync()
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.bufferLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	if t.pipelineLayer != nil {
		t.pipelineLayer.CloseAsync()
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.pipelineLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	t.outputLayer.CloseAsync()
	remaining = timeout - time.Since(started)
	if remaining < 0 {
		return component.ErrTimeout
	}
	if err = t.outputLayer.WaitForClose(remaining); err != nil {
		return
	}

	return nil
}

// StopUnordered attempts to close all components in parallel without allowing
// the stream to gracefully wind down in the order of component layers. This
// should only be attempted if both stopGracefully and stopOrdered failed.
func (t *Type) StopUnordered(timeout time.Duration) (err error) {
	t.inputLayer.CloseAsync()
	if t.bufferLayer != nil {
		t.bufferLayer.CloseAsync()
	}
	if t.pipelineLayer != nil {
		t.pipelineLayer.CloseAsync()
	}
	t.outputLayer.CloseAsync()

	started := time.Now()
	if err = t.inputLayer.WaitForClose(timeout); err != nil {
		return
	}

	var remaining time.Duration

	if t.bufferLayer != nil {
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.bufferLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	if t.pipelineLayer != nil {
		remaining = timeout - time.Since(started)
		if remaining < 0 {
			return component.ErrTimeout
		}
		if err = t.pipelineLayer.WaitForClose(remaining); err != nil {
			return
		}
	}

	remaining = timeout - time.Since(started)
	if remaining < 0 {
		return component.ErrTimeout
	}
	if err = t.outputLayer.WaitForClose(remaining); err != nil {
		return
	}

	return nil
}

// Stop attempts to close the stream within the specified timeout period.
// Initially the attempt is graceful, but as the timeout draws close the attempt
// becomes progressively less graceful.
func (t *Type) Stop(timeout time.Duration) error {
	tOutUnordered := timeout / 4
	tOutGraceful := timeout - tOutUnordered

	err := t.StopGracefully(tOutGraceful)
	if err == nil {
		return nil
	}
	if err == component.ErrTimeout {
		t.manager.Logger().Infoln("Unable to fully drain buffered messages within target time.")
	} else {
		t.manager.Logger().Errorf("Encountered error whilst shutting down: %v\n", err)
	}

	err = t.StopUnordered(tOutUnordered)
	if err == nil {
		return nil
	}
	if err == component.ErrTimeout {
		t.manager.Logger().Errorln("Failed to stop stream gracefully within target time.")

		dumpBuf := bytes.NewBuffer(nil)
		_ = pprof.Lookup("goroutine").WriteTo(dumpBuf, 1)

		t.manager.Logger().Debugln(dumpBuf.String())
	} else {
		t.manager.Logger().Errorf("Encountered error whilst shutting down: %v\n", err)
	}

	return err
}

//------------------------------------------------------------------------------
