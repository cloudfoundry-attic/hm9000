package analyzer

import (
	"github.com/cloudfoundry/hm9000/models"
	"sort"
)

type SortableActualState []models.InstanceHeartbeat

func (s SortableActualState) SortDescendingInPlace() {
	sort.Sort(sort.Reverse(s))
}

func (s SortableActualState) Len() int {
	return len(s)
}

func (s SortableActualState) Less(i, j int) bool {
	return s[i].InstanceIndex < s[j].InstanceIndex
}

func (s SortableActualState) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
