package jitterbug

// jitterbug is a package to implement a lightweight jitter based task system
// The key principal

import (
	rootLog "github.com/rancher-sandbox/scc-operator/internal/log"
	"time"
)

type JitterFunction func(nextTrigger, strictDeadline time.Duration) (bool, error)

func jitterbugContextLogger() rootLog.StructuredLogger {
	logBuilder := rootLog.NewStructuredLoggerBuilder("jitterbug")
	return logBuilder.ToLogger()
}

// JitterChecker is not go-routine safe
type JitterChecker struct {
	log             rootLog.StructuredLogger
	config          *Config
	calculator      Calculator
	callable        JitterFunction
	tickChan        <-chan time.Time
	triggerInterval time.Duration
}

// NewJitterChecker will complete initialization of optional Config fields and return a jitter checker
// It is not a go-routine safe
func NewJitterChecker(config *Config, callable JitterFunction) *JitterChecker {
	calculator := NewJitterCalculator(config, nil)
	return NewJitterCheckerFromCalculator(*calculator, callable)
}

// NewJitterCheckerFromCalculator will complete initialization of optional Config fields and return a jitter checker
func NewJitterCheckerFromCalculator(calculator JitterCalculator, callable JitterFunction) *JitterChecker {
	return &JitterChecker{
		log:        jitterbugContextLogger(),
		config:     calculator.config,
		calculator: &calculator,
		callable:   callable,
	}
}

// Start prepares the first checkin interval and starts the ticker
func (j *JitterChecker) Start() {
	j.calculateCheckinInterval()
	if j.tickChan == nil {
		j.tickChan = time.Tick(j.config.PollingInterval)
	}
}

func (j *JitterChecker) calculateCheckinInterval() {
	j.triggerInterval = j.calculator.CalculateCheckinInterval()
}

func (j *JitterChecker) Run() {
	for range j.tickChan {
		j.log.Debugf("JitterChecker Run: tick")
		j.run()
	}
}

func (j *JitterChecker) run() {
	// Apply initial delay if configured
	if j.config.InitialDelay > 0 {
		j.log.Debugf("[Checker] Initial delay of %s...\n", j.config.InitialDelay)
		select {
		case <-time.After(j.config.InitialDelay):
			// Proceed
		}
	}
	refresh, err := j.callable(j.triggerInterval, j.config.StrictDeadline)
	if err != nil {
		j.log.Errorf("JitterChecker Run-error: %v", err)
		return
	}

	if refresh {
		j.calculateCheckinInterval()
	}
}
