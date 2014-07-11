package peco

import "sort"

type Selection []int

func (s Selection) Has(v int) bool {
	for _, i := range []int(s) {
		if i == v {
			return true
		}
	}
	return false
}

func (s *Selection) Add(v int) {
	if s.Has(v) {
		return
	}
	*s = Selection(append([]int(*s), v))
	sort.Sort(s)
}

func (s *Selection) Remove(v int) {
	a := []int(*s)
	for k, i := range a {
		if i == v {
			tmp := a[:k]
			tmp = append(tmp, a[k+1:]...)
			*s = Selection(tmp)
			return
		}
	}
}

func (s *Selection) Clear() {
	*s = Selection([]int{})
}

func (s Selection) Len() int {
	return len(s)
}

func (s Selection) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Selection) Less(i, j int) bool {
	return s[i] < s[j]
}
