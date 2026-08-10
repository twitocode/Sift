package crawler

type DisjointSet struct {
	parent []int
  rank []int
}

type Cluster struct {
	root *DisjointSet
}

//I dont fully understand this algorithm
func NewDisjointSet(size int) *DisjointSet {
	parent := make([]int, size)
	for i := range size {
		parent[i] = i
	}

	return &DisjointSet{
		parent: parent,
    rank: make([]int, size),
	}
}

func (d *DisjointSet) Find(a int) int {
	if d.parent[a] != a {
		return d.Find(d.parent[a])
	}

	return a
}

func (d *DisjointSet) Union(a int, b int) {
  rootA := d.Find(a)
  rootB := d.Find(b)

  if rootA == rootB {
    return
  }

  if d.rank[rootA] < d.rank[rootB] {
		rootA, rootB = rootB, rootA
	}

	d.parent[rootB] = rootA

	if d.rank[rootA] == d.rank[rootB] {
		d.rank[rootA]++
	}
}
