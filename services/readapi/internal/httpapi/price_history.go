package httpapi

import (
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/model"
)

func computePriceHistoryStats(series []model.PricePoint) priceHistoryStats {
	stats := priceHistoryStats{
		NumObservations: len(series),
	}
	if len(series) == 0 {
		return stats
	}

	first := series[0]
	last := series[len(series)-1]
	stats.FirstObservedAt = first.ObservedAt.Format(timeFormatRFC3339)
	stats.LastObservedAt = last.ObservedAt.Format(timeFormatRFC3339)
	stats.FirstPrice = first.Price
	stats.LastPrice = last.Price
	stats.AbsChange = last.Price - first.Price
	if first.Price != 0 {
		stats.PctChange = (stats.AbsChange / first.Price) * 100
	}

	minPrice := first.Price
	maxPrice := first.Price
	peak := first.Price
	lastChangeAt := time.Time{}

	for i := 1; i < len(series); i++ {
		price := series[i].Price
		if price < minPrice {
			minPrice = price
		}
		if price > maxPrice {
			maxPrice = price
		}

		prevPrice := series[i-1].Price
		if price != prevPrice {
			stats.NumPriceChanges++
			lastChangeAt = series[i].ObservedAt
		}

		if price > peak {
			peak = price
		} else if peak > 0 && price < peak {
			drawdown := ((peak - price) / peak) * 100
			if drawdown > stats.MaxDrawdownPct {
				stats.MaxDrawdownPct = drawdown
			}
		}
	}

	stats.MinPrice = minPrice
	stats.MaxPrice = maxPrice

	if stats.NumPriceChanges > 0 {
		stats.DaysSinceLastChange = int(last.ObservedAt.Sub(lastChangeAt).Hours() / 24)
	}

	return stats
}
