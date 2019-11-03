package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
)

func extendACKs(cfg *ackConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	silences, err := querySilences(ctx, cfg)
	if err != nil {
		return err
	}

	alerts, err := queryAlerts(ctx, cfg)
	if err != nil {
		return err
	}

	metricsSilencesTracked.Set(float64(len(silences)))

	extendIfBefore := time.Now().UTC().Add(cfg.extendIfExpiringIn)

	silencesExpiring := 0
	for _, sil := range silences {
		if !strings.HasPrefix(*sil.Comment, cfg.extendWithPrefix) {
			continue
		}

		usedBy := 0
		for _, alert := range alerts {
			for _, silenceID := range alert.Status.SilencedBy {
				if silenceID == *sil.ID {
					usedBy++
				}
			}
		}
		if usedBy > 0 {
			//fmt.Printf("%s is silencing %d alert(s)\n", *sil.ID, usedBy)
			if time.Time(*sil.EndsAt).Before(extendIfBefore) {
				log.Printf("%s expires in %s and matches %d alert(s), extending it by %s",
					*sil.ID, time.Time(*sil.EndsAt).Sub(time.Now().UTC()), usedBy, cfg.extendBy)
				endsAt := strfmt.DateTime(time.Now().UTC().Add(cfg.extendBy))
				sil.EndsAt = &endsAt
				updateSilence(ctx, cfg, sil)
			}
		} else {
			log.Printf("%s is not used by any alert, letting it expire", *sil.ID)
			silencesExpiring++
		}
	}
	metricsSilencesExpiring.Set(float64(silencesExpiring))

	return nil
}

func ackLoop(cfg *ackConfig) {
	for {
		err := extendACKs(cfg)
		if err != nil {
			log.Printf("Failed to process silences: %s", err)
		}
		metricsCycles.Inc()
		time.Sleep(cfg.loopInterval)
	}
}
