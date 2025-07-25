package controllers

import (
	"github.com/rancher-sandbox/scc-operator/internal/util"
	v1 "github.com/rancher-sandbox/scc-operator/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher-sandbox/scc-operator/pkg/util/jitterbug"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

func setupCfg() *jitterbug.Config {
	// Configure jitter based daily revalidation trigger
	jitterbugConfig := jitterbug.Config{
		BaseInterval:    prodBaseCheckin,
		JitterMax:       3,
		JitterMaxScale:  time.Hour,
		PollingInterval: 9 * time.Minute,
	}
	if util.DevMode() {
		jitterbugConfig = jitterbug.Config{
			BaseInterval:    devBaseCheckin,
			JitterMax:       10,
			JitterMaxScale:  time.Minute,
			PollingInterval: 9 * time.Second,
		}
	}
	return &jitterbugConfig
}

func (h *handler) RunLifecycleManager(
	cfg *jitterbug.Config,
) {
	// min jitter 20 hours
	jitterCheckin := jitterbug.NewJitterChecker(
		cfg,
		func(nextTrigger, strictDeadline time.Duration) (bool, error) {
			registrationsCacheList, err := h.registrationCache.List(labels.Everything())
			if err != nil {
				h.log.Errorf("Failed to list registrations: %v", err)
				return false, err
			}

			checkInWasTriggered := false
			for _, registrationObj := range registrationsCacheList {
				registrationHandler := h.prepareHandler(registrationObj)

				// Always skip offline mode registrations, or Registrations that haven't progressed to activation
				if registrationObj.Spec.Mode == v1.RegistrationModeOffline ||
					registrationHandler.NeedsRegistration(registrationObj) ||
					registrationObj.Status.ActivationStatus.LastValidatedTS.IsZero() {
					continue
				}

				lastValidated := registrationObj.Status.ActivationStatus.LastValidatedTS

				timeSinceLastValidation := time.Since(lastValidated.Time)
				// If the time since last validation is after the daily trigger (which includes jitter), we revalidate.
				// Also, ensure that when a registration is over the strictDeadline it is checked.
				if timeSinceLastValidation >= nextTrigger || timeSinceLastValidation >= strictDeadline {
					checkInWasTriggered = true
					syncNowReg := registrationObj.DeepCopy()
					syncNow := true
					syncNowReg.Spec.SyncNow = &syncNow
					_, err := h.registrations.Update(syncNowReg)
					if err != nil {
						return true, err
					}
				}
			}

			return checkInWasTriggered, nil
		},
	)
	jitterCheckin.Start()
	jitterCheckin.Run()
}
