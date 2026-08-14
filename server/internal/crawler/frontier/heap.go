package frontier

import (
	"errors"
	"slices"
)

//Help: https://www.youtube.com/watch?v=t0Cq6tVNRBA

// Heaps are a special type of binary tree in which they can be easily represented by an array because of the insertion order of items
// This is a min heap
type CooldownHeap struct {
	values []*BQueue
	size   int
}

func NewHeap(capacity int) *CooldownHeap {
	return &CooldownHeap{
		values: make([]*BQueue, 0, capacity),
		size:   0,
	}
}

func (mh *CooldownHeap) EnsureCapacity() {
	if mh.size >= cap(mh.values) {
		mh.values = slices.Grow(mh.values, 2*cap(mh.values))
	}
}

// Gets the root of the heap
func (mh *CooldownHeap) Peek() (*BQueue, error) {
	if mh.size > 0 {
		return mh.values[0], nil
	}

	return nil, errors.New("heap is empty")
}

// Extracts minimum element
func (mh *CooldownHeap) Poll() (*BQueue, error) {
	if mh.size == 0 {
		return nil, errors.New("heap is empty")
	}

	item := mh.values[0]
	mh.values[0] = mh.values[mh.size-1]
	mh.values = mh.values[1:]

	mh.HeapifyDown()
	return item, nil
}

func (mh *CooldownHeap) Add(host *BQueue) {
	mh.EnsureCapacity()
	mh.values = append(mh.values, host)
	mh.size++

	mh.HeapifyUp()
}

func (mh *CooldownHeap) HeapifyDown() {
	//start at root node
	index := 0

	//if there is no left then there is definitely no child
	for mh.HasLeft(index) {
		smallerChildIndex := mh.getLeftIndex(index)
		if mh.HasRight(index) && mh.GetRight(index).NextEligibleAt.Before(mh.GetLeft(index).NextEligibleAt) {
			smallerChildIndex = mh.getRightIndex(index)
		}

		//if i am smaller than my children
		if mh.values[index].NextEligibleAt.Before(mh.values[smallerChildIndex].NextEligibleAt) {
			break
		}

		mh.values[index], mh.values[smallerChildIndex] = mh.values[smallerChildIndex], mh.values[index]
		index = smallerChildIndex
	}
}

func (mh *CooldownHeap) HeapifyUp() {
	//start at last node
	index := mh.size - 1

	for mh.HasParent(index) && mh.GetParent(index).NextEligibleAt.After(mh.values[index].NextEligibleAt) {
		parent := mh.getParentIndex(index)
		mh.values[parent], mh.values[index] = mh.values[index], mh.values[parent]

		//walking upwards
		index = parent
	}
}

func (mh *CooldownHeap) getLeftIndex(parentIndex int) int {
	return 2*parentIndex + 1
}

func (mh *CooldownHeap) getRightIndex(parentIndex int) int {
	return 2*parentIndex + 2
}

func (mh *CooldownHeap) getParentIndex(childIndex int) int {
	return (childIndex - 1) / 2
}

func (mh *CooldownHeap) HasLeft(index int) bool {
	return mh.getLeftIndex(index) < mh.size
}

func (mh *CooldownHeap) HasRight(index int) bool {
	return mh.getRightIndex(index) < mh.size
}

func (mh *CooldownHeap) HasParent(index int) bool {
	return mh.getParentIndex(index) >= 0
}

func (mh *CooldownHeap) GetLeft(index int) *BQueue {
	return mh.values[mh.getLeftIndex(index)]
}

func (mh *CooldownHeap) GetRight(index int) *BQueue {
	return mh.values[mh.getRightIndex(index)]
}

func (mh *CooldownHeap) GetParent(index int) *BQueue {
	return mh.values[mh.getParentIndex(index)]
}
