package db

import "testing"

type mockIterator struct {
	data []Item
	pos  uint
	err  error
}

func (i *mockIterator) Next() bool {
	if i.pos == uint(len(i.data)) {
		return false
	}
	i.pos++
	return true
}

func (i *mockIterator) Error() error {
	return i.err
}

func (i *mockIterator) Key() []byte {
	return i.data[i.pos].Key
}

func (i *mockIterator) Value() []byte {
	return i.data[i.pos].Value
}

func (i *mockIterator) Release() {
	i.data = make([]Item, 0)
	i.pos = 0
	i.err = nil
}

func TestChanIterator(t *testing.T) {
	data := []Item{
		Item{[]byte{0}, []byte{4}},
		Item{[]byte{1}, []byte{5, 6}},
		Item{[]byte{2}, []byte{7, 8, 9}},
	}

	it := mockIterator{data: data}
	cit := AsChanIterator(it)
	defer func() { cit.Done() <- struct{}{} }()

	i := uint(0)
	for item := range cit.Items() {
		// do something with item
		if !data[i].Equal(item) {
			t.Errorf("Expected: %v, got %v", data[i], item)
		}
		i++
	}
	if err := cit.Error(); err != nil {
		// handle error
		t.Errorf("Iterator error: $v", err)
	}
}
