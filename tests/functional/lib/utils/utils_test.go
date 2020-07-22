package utils

import (
	"reflect"
	"testing"
)

func TestMakeRange(t *testing.T) {
	list, err := makeRange(0, 10, 1)
	if err != nil {
		t.Errorf("makeRange(0, 10, 1) resulted in: %s", err)
	}
	expected := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("got %v, expected %v", list, expected)
	}

	list, err = makeRange(0, 10, 2)
	if err != nil {
		t.Errorf("makeRange(0, 10, 2) resulted in: %s", err)
	}
	expected = []int{0, 2, 4, 6, 8}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("got %v, expected %v", list, expected)
	}

	list, err = makeRange(5, 10, 2)
	if err != nil {
		t.Errorf("makeRange(5, 10, 2) resulted in: %s", err)
	}
	expected = []int{5, 7, 9}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("got %v, expected %v", list, expected)
	}

	list, err = makeRange(10, 0, -1)
	if err != nil {
		t.Errorf("makeRange(10, 0, -1) resulted in: %s", err)
	}
	expected = []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("got %v, expected %v", list, expected)
	}

	list, err = makeRange(0, 0, 0)
	if err == nil {
		t.Errorf("makeRange(0, 0, 0) expected error, generated: %v", list)
	}

	list, err = makeRange(0, 10, -1)
	if err == nil {
		t.Errorf("makeRange(0, 10, -1) expected error, generated: %v", list)
	}
	list, err = makeRange(10, 0, 1)
	if err == nil {
		t.Errorf("makeRange(10, 0, 1) expected error, generated: %v", list)
	}
}
