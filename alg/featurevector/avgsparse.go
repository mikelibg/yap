package featurevector

import (
	"fmt"
	// "log"
	"strings"
	"sync"
)

type HistoryValue struct {
	sync.Mutex
	Generation     int
	PrevGeneration int
	Value, Total   int64
}

func (h *HistoryValue) Integrate(generation int) {
	h.Value = h.Total + (int64)(generation-h.Generation)*h.Value
}

func (h *HistoryValue) Add(generation int, amount int64) {
	h.Lock()
	defer h.Unlock()
	if h.PrevGeneration < h.Generation {
		h.Total += (int64)(generation-h.Generation) * h.Value
	}
	if h.Generation < generation {
		h.PrevGeneration, h.Generation = h.Generation, generation
	}
	h.Value = h.Value + amount
}

func NewHistoryValue(generation int, value int64) *HistoryValue {
	return &HistoryValue{Generation: generation, Value: value}
}

type TransitionScoreKVFunc func(key int, value *HistoryValue)

type TransitionScoreStore interface {
	Add(generation, transition int, feature interface{}, amount int64)
	Integrate(generation int)
	Len() int
	SetValue(key int, value *HistoryValue)
	GetValue(key int) *HistoryValue
	Each(f TransitionScoreKVFunc)
}

type LockedArray struct {
	sync.RWMutex
	Vals []*HistoryValue
}

var _ TransitionScoreStore = &LockedArray{}

func (l *LockedArray) ExtendFor(generation, transition int) {
	newVals := make([]*HistoryValue, transition+1)
	copy(newVals[0:len(l.Vals)], l.Vals[0:len(l.Vals)])
	l.Vals = newVals
}

func (l *LockedArray) Add(generation, transition int, feature interface{}, amount int64) {
	l.Lock()
	defer l.Unlock()
	if transition < len(l.Vals) {
		if l.Vals[transition] != nil {
			l.Vals[transition].Add(generation, amount)
		} else {
			l.Vals[transition] = NewHistoryValue(generation, amount)
		}
		return
	} else {
		l.ExtendFor(generation, transition)
		if transition >= len(l.Vals) {
			panic("Despite extending, transition >= than Vals")
		}
		l.Vals[transition] = NewHistoryValue(generation, amount)
		return
	}
}

func (l *LockedArray) SetValue(key int, value *HistoryValue) {
	l.Vals[key] = value
}

func (l *LockedArray) GetValue(key int) *HistoryValue {
	if key < len(l.Vals) {
		return l.Vals[key]
	} else {
		return nil
	}
}

func (l *LockedArray) Integrate(generation int) {
	for _, v := range l.Vals {
		if v != nil {
			v.Integrate(generation)
		}
	}
}

func (l *LockedArray) Len() int {
	return len(l.Vals)
}

func (l *LockedArray) Each(f TransitionScoreKVFunc) {
	for i, hist := range l.Vals {
		f(i, hist)
	}
}

type LockedMap struct {
	sync.RWMutex
	Vals map[int]*HistoryValue
}

var _ TransitionScoreStore = &LockedMap{}

func (l *LockedMap) Add(generation, transition int, feature interface{}, amount int64) {
	l.Lock()
	defer l.Unlock()

	if historyValue, ok := l.Vals[transition]; ok {
		historyValue.Add(generation, amount)
		return
	} else {
		l.Vals[transition] = NewHistoryValue(generation, amount)
		return
	}
}

func (l *LockedMap) Integrate(generation int) {
	for _, v := range l.Vals {
		v.Integrate(generation)
	}
}

func (l *LockedMap) Len() int {
	return len(l.Vals)
}

func (l *LockedMap) SetValue(key int, value *HistoryValue) {
	l.Vals[key] = value
}

func (l *LockedMap) GetValue(key int) *HistoryValue {
	if value, exists := l.Vals[key]; exists {
		return value
	} else {
		return nil
	}
}

func (l *LockedMap) Each(f TransitionScoreKVFunc) {
	for i, hist := range l.Vals {
		f(i, hist)
	}
}

type AvgSparse struct {
	sync.RWMutex
	Dense bool
	Vals  map[Feature]TransitionScoreStore
}

func (v *AvgSparse) Value(transition int, feature interface{}) int64 {
	transitions, exists := v.Vals[feature]
	if exists && transition < transitions.Len() {
		if histValue := transitions.GetValue(transition); histValue != nil {
			return histValue.Value
		}
	}
	return 0.0
}

func (v *AvgSparse) Add(generation, transition int, feature interface{}, amount int64, wg *sync.WaitGroup) {
	v.Lock()
	defer v.Unlock()
	transitions, exists := v.Vals[feature]
	if exists {
		// wg.Add(1)
		go func() {
			transitions.Add(generation, transition, feature, amount)
			wg.Done()
		}()
	} else {
		newTrans := &LockedArray{Vals: make([]*HistoryValue, transition+1)}
		newTrans.Vals[transition] = NewHistoryValue(generation, amount)
		if v.Vals == nil {
			panic("Got nil Vals")
		}
		v.Vals[feature] = newTrans
		wg.Done()
	}
}

func (v *AvgSparse) Integrate(generation int) *AvgSparse {
	for _, val := range v.Vals {
		val.Integrate(generation)
	}
	return v
}

func (v *AvgSparse) SetScores(feature Feature, scores *[]int64) {
	transitions, exists := v.Vals[feature]
	if exists {
		// log.Println("\t\tSetting scores for feature", feature)
		// log.Println("\t\t\t1. Exists")
		transitionsLen := transitions.Len()
		if cap(*scores) < transitionsLen {
			// log.Println("\t\t\t1.1 Scores array not large enough")
			newscores := make([]int64, transitionsLen)
			// log.Println("\t\t\t1.2 Copying")
			copy(newscores[0:transitionsLen], (*scores)[0:len(*scores)])
			// log.Println("\t\t\t1.3 Setting pointer")
			*scores = newscores
		}
		// log.Println("\t\t\t2. Iterating", len(transitions), "transitions")
		transitions.Each(func(i int, val *HistoryValue) {
			if val == nil {
				return
			}
			// log.Println("\t\t\t\tAt transition", i)
			for len(*scores) <= i {
				// log.Println("\t\t\t\t2.2 extending scores of len", len(*scores), "up to", i)
				*scores = append(*scores, 0)
			}
			// log.Println("\t\t\t\t2.3 incrementing with", val.Value)
			(*scores)[i] += val.Value

		})
		// for i, val := range transitions.Values() {
		// 	if val == nil {
		// 		continue
		// 	}
		// 	// log.Println("\t\t\t\tAt transition", i)
		// 	for len(*scores) <= i {
		// 		// log.Println("\t\t\t\t2.2 extending scores of len", len(*scores), "up to", i)
		// 		*scores = append(*scores, 0)
		// 	}
		// 	// log.Println("\t\t\t\t2.3 incrementing with", val.Value)
		// 	(*scores)[i] += val.Value
		// }
		// log.Println("\t\tReturning scores array", *scores)
	}
}

func (v *AvgSparse) UpdateScalarDivide(byValue int64) *AvgSparse {
	if byValue == 0.0 {
		panic("Divide by 0")
	}
	v.RLock()
	defer v.RUnlock()
	for _, val := range v.Vals {
		val.Each(func(i int, histValue *HistoryValue) {
			histValue.Value = histValue.Value / byValue
		})
	}
	return v
}

func (v *AvgSparse) String() string {
	strs := make([]string, 0, len(v.Vals))
	v.RLock()
	defer v.RUnlock()
	for feat, val := range v.Vals {
		strs = append(strs, fmt.Sprintf("%v %v", feat, val))
	}
	return strings.Join(strs, "\n")
}

func (v *AvgSparse) Serialize() interface{} {
	// retval := make(map[interface{}][]int64, len(v.Vals))
	retval := make(map[interface{}][]int64, len(v.Vals))
	for k, v := range v.Vals {
		scores := make([]int64, v.Len())
		v.Each(func(i int, lastScore *HistoryValue) {
			if lastScore != nil {
				scores[i] = lastScore.Value
			}
		})
		// for i, lastScore := range v.Vals {
		// 	if lastScore != nil {
		// 		scores[i] = lastScore.Value
		// 	}
		// }
		retval[k] = scores
	}
	return retval
}

func (v *AvgSparse) Deserialize(serialized interface{}, generation int) {
	data, ok := serialized.(map[interface{}][]int64)
	if !ok {
		panic("Can't deserialize unknown serialization")
	}
	v.Vals = make(map[Feature]TransitionScoreStore, len(data))
	for k, datav := range data {
		scoreStore := v.newTransitionScoreStore(len(datav))
		for i, value := range datav {
			scoreStore.SetValue(i, NewHistoryValue(generation, value))
		}
		v.Vals[k] = scoreStore
	}
}

func (v *AvgSparse) newTransitionScoreStore(size int) TransitionScoreStore {
	if v.Dense {
		return &LockedArray{Vals: make([]*HistoryValue, size)}
	} else {
		return &LockedMap{Vals: make(map[int]*HistoryValue, size)}
	}
}

func NewAvgSparse() *AvgSparse {
	return MakeAvgSparse(false)
}

func MakeAvgSparse(dense bool) *AvgSparse {
	return &AvgSparse{Vals: make(map[Feature]TransitionScoreStore, 100), Dense: dense}
}