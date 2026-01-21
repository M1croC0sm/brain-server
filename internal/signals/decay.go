package signals

import (
	"math"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
)

// Half-lives in days
const (
	HalfLifeTerm     = 3.0
	HalfLifeCategory = 7.0
	HalfLifeProject  = 30.0
)

// Floors - PROJECTS ONLY (allows "restart is always allowed" for themes)
const (
	FloorProject = 0.02
)

// Boost values for signal updates
const (
	BoostTerm     = 1.0
	BoostCategory = 0.5
	BoostProject  = 1.0
)

// Caps to prevent runaway weights
const (
	CapTerm     = 10.0
	CapCategory = 5.0
	CapProject  = 10.0
)

// lambda computes decay constant: λ = ln(2) / half_life
func lambda(halfLife float64) float64 {
	return 0.693147 / halfLife
}

// getHalfLife returns the half-life for a signal type
func getHalfLife(signalType string) float64 {
	switch signalType {
	case "term":
		return HalfLifeTerm
	case "category":
		return HalfLifeCategory
	case "project":
		return HalfLifeProject
	default:
		return HalfLifeTerm // default to shortest
	}
}

// getCap returns the cap for a signal type
func getCap(signalType string) float64 {
	switch signalType {
	case "term":
		return CapTerm
	case "category":
		return CapCategory
	case "project":
		return CapProject
	default:
		return CapTerm
	}
}

// getBoost returns the boost value for a signal type
func getBoost(signalType string) float64 {
	switch signalType {
	case "term":
		return BoostTerm
	case "category":
		return BoostCategory
	case "project":
		return BoostProject
	default:
		return BoostTerm
	}
}

// DecayWeight applies exponential decay to a weight
// newWeight = oldWeight * exp(-λ * Δdays)
// Applies floor only for projects with ever_dominant flag
func DecayWeight(oldWeight float64, daysSince float64, signalType string, everDominant bool) float64 {
	halfLife := getHalfLife(signalType)
	lam := lambda(halfLife)
	newWeight := oldWeight * math.Exp(-lam*daysSince)

	// Apply floor only for dominant projects
	if signalType == "project" && everDominant && newWeight < FloorProject {
		newWeight = FloorProject
	}

	return newWeight
}

// DecayAllSignals runs decay on all signals in the database
// This should be called daily before letter generation
func DecayAllSignals(database *db.DB) error {
	signals, err := database.GetAllSignals()
	if err != nil {
		return err
	}

	now := time.Now()

	for _, s := range signals {
		daysSince := now.Sub(s.LastUpdated).Hours() / 24.0
		if daysSince <= 0 {
			continue // Already updated today
		}

		newWeight := DecayWeight(s.Weight, daysSince, s.Type, s.EverDominant)

		// Delete signals that have decayed to effectively zero (< 0.001)
		// Exception: dominant projects keep their floor
		if newWeight < 0.001 && !(s.Type == "project" && s.EverDominant) {
			if err := database.DeleteSignal(s.Key); err != nil {
				return err
			}
			continue
		}

		if err := database.UpdateSignalWeight(s.Key, newWeight); err != nil {
			return err
		}
	}

	return nil
}

// BoostSignal applies a boost to a signal with lazy decay
// It first decays the existing weight, then adds the boost
func BoostSignal(database *db.DB, key, signalType string) error {
	existing, err := database.GetSignal(key)
	if err != nil {
		return err
	}

	boost := getBoost(signalType)
	cap := getCap(signalType)

	var newWeight float64
	if existing == nil {
		// New signal, just use the boost
		newWeight = boost
	} else {
		// Decay existing weight first
		daysSince := time.Since(existing.LastUpdated).Hours() / 24.0
		decayedWeight := DecayWeight(existing.Weight, daysSince, signalType, existing.EverDominant)
		newWeight = decayedWeight + boost
	}

	// Apply cap
	if newWeight > cap {
		newWeight = cap
	}

	return database.UpsertSignal(key, signalType, newWeight)
}
