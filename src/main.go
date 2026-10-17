package main

import (
	"fmt"
	"strings"
	"sync"
)

func main() {
  fmt.Println("hello, world!");
  var n int
  fmt.Println(n)
  var p *int
  fmt.Println(p)
  var xs []int
  fmt.Println(xs)
  var m map[string]int
  fmt.Println(m)
  var sb strings.Builder
  fmt.Println(sb)
  var mu sync.Mutex
  fmt.Println(mu)
}
