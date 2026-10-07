package lvr

import (
	"slices"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

const (
	outlierFence      = 3.0
	outlierMinSamples = 20
)

// MarkPriceOutliers flags transactions whose price per ping lies beyond 3 IQRs of their
// district's quartiles. Special transactions are excluded from the quartiles; districts
// with fewer than 20 samples are left untouched.
func MarkPriceOutliers(transactions []domain.Transaction) {
	flags := outlierFlags(len(transactions), func(index int) (string, float64, bool, bool) {
		transaction := transactions[index]
		return transaction.CityCode + transaction.District, transaction.PricePerPing, !transaction.IsSpecial(), true
	})
	for index, outlier := range flags {
		if outlier && !slices.Contains(transactions[index].SpecialReasons, SpecialPriceOutlier) {
			transactions[index].SpecialReasons = append(transactions[index].SpecialReasons, SpecialPriceOutlier)
		}
	}
}

// outlierFlags reports which of n values lie beyond outlierFence IQRs of their group's
// quartiles. item returns a value's group, the value, whether it counts toward the
// quartiles and whether it is checked. Groups with fewer than outlierMinSamples values
// are not checked.
func outlierFlags(n int, item func(index int) (group string, value float64, sample, check bool)) []bool {
	samples := map[string][]float64{}
	for index := range n {
		if group, value, sample, _ := item(index); sample {
			samples[group] = append(samples[group], value)
		}
	}

	type fence struct{ low, high float64 }
	fences := make(map[string]fence, len(samples))
	for group, values := range samples {
		if len(values) < outlierMinSamples {
			continue
		}
		slices.Sort(values)
		q1, q3 := quantile(values, 0.25), quantile(values, 0.75)
		iqr := q3 - q1
		fences[group] = fence{low: q1 - outlierFence*iqr, high: q3 + outlierFence*iqr}
	}

	flags := make([]bool, n)
	for index := range n {
		group, value, _, check := item(index)
		if bounds, ok := fences[group]; ok && check {
			flags[index] = value < bounds.low || value > bounds.high
		}
	}
	return flags
}

// MarkInvalidDates flags sales dated after latest, the last day their registration could
// have been made: a sale is registered after it happens, so a later date is a typo.
func MarkInvalidDates(transactions []domain.Transaction, latest time.Time) {
	for index := range transactions {
		transaction := &transactions[index]
		if transaction.TransactedAt.After(latest) && !slices.Contains(transaction.SpecialReasons, SpecialInvalidDate) {
			transaction.SpecialReasons = append(transaction.SpecialReasons, SpecialInvalidDate)
		}
	}
}

// quantile uses linear interpolation between closest ranks of sorted values.
func quantile(sorted []float64, fraction float64) float64 {
	position := fraction * float64(len(sorted)-1)
	lower := int(position)
	if lower+1 >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (position-float64(lower))*(sorted[lower+1]-sorted[lower])
}
