package main

import (
	"fmt"
	"testing"
)

func TestParseIP(t *testing.T) {
	s := "10.4.0.0/20"
	addr := parseIP(s, s)

	if addr.v4 != 168034304 {
		t.Fatalf("got: %d, expected: %d", addr.v4, 168034304)
	}

}

func TestTreeBits(t *testing.T) {
	xs := []string{"10.4.16.0/20", "10.5.16.0/20", "10.4.0.0/20", "10.0.0.0/8"}
	for _, x := range xs {

		addr := parseIP(x, x)
		fmt.Printf("%14s %s\n", x, bits(addr.v4))
		fmt.Println(addr.Unmasked())
	}
}

func TestFindInEmptyAsc(t *testing.T) {

	network := New("10.0.0.0/8")

	expected := []string{
		"10.0.0.0/16",
		"10.1.0.0/17",
		"10.1.128.0/18",
		"10.1.192.0/19",
		"10.1.224.0/20",
		"10.1.240.0/21",
		"10.1.248.0/22",
		"10.1.252.0/23",
		"10.1.254.0/24",
		"10.1.255.0/25",
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-%d", i+16)
		addr := network.FindSmallest(i + 16)
		addr.name = name
		//fmt.Println(addr, expected[i])
		if addr.String() != expected[i] {
			t.Fatalf("expected: %s, got: %s", expected[i], addr)
		}
		network.Insert(addr)
	}
	network.Print()
}

func TestFindInEmptySmallFirst(t *testing.T) {
	network := New("10.0.0.0/8")

	expected := []string{
		"10.0.0.0/25",
		"10.0.0.128/25",
		"10.0.1.0/24",
		"10.0.2.0/23",
		"10.0.4.0/22",
		"10.0.8.0/21",
		"10.0.16.0/20",
		"10.0.32.0/19",
		"10.0.64.0/18",
		"10.0.128.0/17",
		"10.1.0.0/16",
	}

	for i, n := range []int{25, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16} {
		name := fmt.Sprintf("network-%d", n)
		addr := network.FindSmallest(n)
		if addr.String() != expected[i] {
			t.Fatalf("expected: %s, got: %s", expected[i], addr)
		}
		addr.name = name
		network.Insert(addr)
	}

	network.Print()
}
