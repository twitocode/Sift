package ranker

import (
	"math"

	"github.com/twitocode/sift/internal/common"
)

func CalculateBM25(docWithTerm int, terms []string, docTokenCount uint32, docTermFreq uint32, indexStats common.IndexStats) float64 {
	score := float64(0)

	fqdi := docTermFreq
	nqi := float64(docWithTerm)
	IDF := math.Log((float64(indexStats.DocumentCount)-nqi+0.5)/(nqi+0.5) + 1)
	D := docTokenCount
	avgdl := indexStats.AverageDocLength
	k := 1.35
	b := 0.75

	if avgdl == 0 {
		return 0
	}

	score += IDF * (float64(fqdi) * (k + 1)) / (float64(fqdi) + (k * ((b * (float64(D) / avgdl)) + 1 - b)))

	return score
}
