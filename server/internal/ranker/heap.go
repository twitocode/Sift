package ranker

import (
	"errors"
)

type idScore struct {
	id    int32
	score float64
}

// time complexity is O(log 10)
type BestCandidateHeap struct {
	values   []*idScore
	size     int
	capacity int
}

func NewBestCandidateHeap(capacity int) *BestCandidateHeap {
	if capacity < 0 {
		capacity = 0
	}

	return &BestCandidateHeap{
		values:   make([]*idScore, 0, capacity),
		capacity: capacity,
	}
}

// NewCooldownHeap is kept as an alias for callers using the old constructor.
func NewCooldownHeap(capacity int) *BestCandidateHeap {
	return NewBestCandidateHeap(capacity)
}

func (bch *BestCandidateHeap) Peek() (*idScore, error) {
	if bch.size > 0 {
		return bch.values[0], nil
	}

	return nil, errors.New("heap is empty")
}

func (bch *BestCandidateHeap) Poll() (*idScore, error) {
	if bch.size == 0 {
		return nil, errors.New("heap is empty")
	}

	item := bch.values[0]
	last := bch.size - 1

	bch.values[0] = bch.values[last]
	bch.values[last] = nil
	bch.values = bch.values[:last]
	bch.size--

	if bch.size > 0 {
		bch.HeapifyDown()
	}

	return item, nil
}

func (bch *BestCandidateHeap) Add(candidate *idScore) {
	if bch.capacity == 0 {
		return
	}

	if bch.size < bch.capacity {
		bch.values = append(bch.values, candidate)
		bch.size++
		bch.HeapifyUp()
		return
	}

	// The root is the lowest-scoring retained candidate. Keep the heap
	// bounded by replacing it only when the new candidate is better.
	if candidate.score <= bch.values[0].score {
		return
	}

	bch.values[0] = candidate
	bch.HeapifyDown()
}

func (bch *BestCandidateHeap) HeapifyDown() {
	//start at root node
	index := 0

	//if there is no left then there is definitely no child
	for bch.HasLeft(index) {
		smallerChildIndex := bch.getLeftIndex(index)
		if bch.HasRight(index) && bch.GetRight(index).score < bch.GetLeft(index).score {
			smallerChildIndex = bch.getRightIndex(index)
		}

		//if i am smaller than my children
		if bch.values[index].score < bch.values[smallerChildIndex].score {
			break
		}

		bch.values[index], bch.values[smallerChildIndex] = bch.values[smallerChildIndex], bch.values[index]
		index = smallerChildIndex
	}
}

func (bch *BestCandidateHeap) HeapifyUp() {
	//start at last node
	index := bch.size - 1

	for bch.HasParent(index) && bch.GetParent(index).score > bch.values[index].score {
		parent := bch.getParentIndex(index)
		bch.values[parent], bch.values[index] = bch.values[index], bch.values[parent]

		//walking upwards
		index = parent
	}
}

func (bch *BestCandidateHeap) getLeftIndex(parentIndex int) int {
	return 2*parentIndex + 1
}

func (bch *BestCandidateHeap) getRightIndex(parentIndex int) int {
	return 2*parentIndex + 2
}

func (bch *BestCandidateHeap) getParentIndex(childIndex int) int {
	return (childIndex - 1) / 2
}

func (bch *BestCandidateHeap) HasLeft(index int) bool {
	return bch.getLeftIndex(index) < bch.size
}

func (bch *BestCandidateHeap) HasRight(index int) bool {
	return bch.getRightIndex(index) < bch.size
}

func (bch *BestCandidateHeap) HasParent(index int) bool {
	return bch.getParentIndex(index) >= 0
}

func (bch *BestCandidateHeap) GetLeft(index int) *idScore {
	return bch.values[bch.getLeftIndex(index)]
}

func (bch *BestCandidateHeap) GetRight(index int) *idScore {
	return bch.values[bch.getRightIndex(index)]
}

func (bch *BestCandidateHeap) GetParent(index int) *idScore {
	return bch.values[bch.getParentIndex(index)]
}
