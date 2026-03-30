package dag

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NodeContext is passed to every ExecuteFunc.
// It carries the parent context and a node-scoped logger.
type NodeContext struct {
	context.Context
	log *zap.Logger
}

// Log returns the node-scoped zap logger.
// All messages are tagged with the node ID and streamed to the LogSink (if set).
func (nc *NodeContext) Log() *zap.Logger {
	return nc.log
}

// WithContext returns a shallow copy of NodeContext with a different context.Context.
// The logger is preserved from the original.
func (nc *NodeContext) WithContext(ctx context.Context) *NodeContext {
	return &NodeContext{Context: ctx, log: nc.log}
}

// LogSink receives log entries per node for external streaming (UI, storage, etc.).
type LogSink interface {
	// WriteLog is called for every log entry produced by a node.
	WriteLog(ctx context.Context, executionID string, nodeID string, entry zapcore.Entry, fields []zapcore.Field)
}

// sinkCore is a zapcore.Core that forwards entries to a LogSink.
type sinkCore struct {
	executionID string
	nodeID      string
	sink        LogSink
	fields      []zapcore.Field
	level       zapcore.Level
}

func (c *sinkCore) Enabled(lvl zapcore.Level) bool { return lvl >= c.level }

func (c *sinkCore) With(fields []zapcore.Field) zapcore.Core {
	return &sinkCore{
		executionID: c.executionID,
		nodeID:      c.nodeID,
		sink:        c.sink,
		fields:      append(append([]zapcore.Field{}, c.fields...), fields...),
		level:       c.level,
	}
}

func (c *sinkCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		ce = ce.AddCore(entry, c)
	}
	return ce
}

func (c *sinkCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	all := append(append([]zapcore.Field{}, c.fields...), fields...)
	c.sink.WriteLog(context.Background(), c.executionID, c.nodeID, entry, all)
	return nil
}

func (c *sinkCore) Sync() error { return nil }

// newNodeLogger builds a logger for a specific node.
// It tees output to the base logger and the optional LogSink.
func newNodeLogger(base *zap.Logger, sink LogSink, executionID, nodeID string) *zap.Logger {
	nodeLogger := base.With(zap.String("node_id", nodeID))

	if sink == nil {
		return nodeLogger
	}

	sc := &sinkCore{
		executionID: executionID,
		nodeID:      nodeID,
		sink:        sink,
		level:       zapcore.DebugLevel,
	}

	tee := zapcore.NewTee(nodeLogger.Core(), sc)
	return zap.New(tee).With(zap.String("node_id", nodeID))
}
