package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1beta1"
)

var bitmask []uint

func init() {
	bitmask = make([]uint, 32)
	for i := 0; i < 32; i++ {
		bitmask[i] = 1 << (31 - i)
	}
}

func networkShortName(s string) string {
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

func parseByte(b string) uint {
	u, err := strconv.ParseUint(b, 10, 64)
	if err != nil {
		panic(err)
	}

	return uint(u)
}

func parseIP(name, ip string) *Address {
	parts := strings.Split(ip, "/")
	if len(parts) != 2 {
		panic("unexpected address format " + ip)
	}

	mask, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(err)
	}

	address := strings.Split(parts[0], ".")
	if len(address) != 4 {
		panic("unexpected address format " + ip)
	}

	var r uint
	for i := 0; i < 4; i++ {
		r = (r << 8) | parseByte(address[i])
	}

	return &Address{r, mask, name}
}

type Address struct {
	v4   uint
	mask int
	name string
}

func (a *Address) Unmasked() uint {
	shift := 32 - a.mask
	return (a.v4 >> (shift)) << shift
}

func formatV4(r uint, mask int) string {
	parts := make([]string, 4)
	for i := 3; i >= 0; i-- {
		n := r & 255
		r = r >> 8
		parts[i] = fmt.Sprintf("%d", n)
	}

	return fmt.Sprintf("%s/%d", strings.Join(parts, "."), mask)
}

func (a Address) String() string {
	return formatV4(a.v4, a.mask)
}

type Tree struct {
	L   *Tree
	R   *Tree
	Val *Address
}

func (n *Network) Print() {
	printSubnets(n.root, 0, 0)
}

func printSubnets(root *Tree, indent, depth int) {
	if root == nil {
		return
	}

	if root.Val != nil {
		fmt.Printf(
			"%s %s %s (%s)\n",
			strings.Repeat("  ", indent),
			strings.Repeat("--", depth-indent),
			root.Val.name,
			root.Val.String(),
		)
	}

	if root.L != nil && root.R != nil {
		indent = depth
	}

	printSubnets(root.L, indent, depth+1)
	printSubnets(root.R, indent, depth+1)
}

func New(root string) *Network {
	return &Network{
		prefix: parseIP("root", root),
	}
}

type Network struct {
	prefix *Address
	root   *Tree
}

func (n *Network) Insert(addr *Address) {
	n.root = insert(n.root, addr, n.prefix.mask)
}

func bits(u uint) string {
	xs := []string{}
	for i := 1; i <= 32; i++ {
		bit := (u >> (32 - i)) & 1
		s := strconv.FormatUint(uint64(bit), 10)
		xs = append(xs, s)
	}

	return strings.Join(xs, ",")
}

func insert(root *Tree, a *Address, depth int) *Tree {
	if depth == a.mask {
		if root != nil {
			panic("child ip range already exists")
		}
		return &Tree{
			Val: a,
		}
	}

	if root == nil {
		root = &Tree{}
	}

	if root.Val != nil {
		panic("val != nil")
	}

	p := a.v4 & bitmask[depth]
	if p == 0 {
		return &Tree{
			L:   insert(root.L, a, depth+1),
			R:   root.R,
			Val: root.Val,
		}
	}
	return &Tree{
		L:   root.L,
		R:   insert(root.R, a, depth+1),
		Val: root.Val,
	}
}

type match struct {
	path  uint
	depth int
}

func (n *Network) FindSmallest(desiredMask int) *Address {
	matches := findSmallest(n.root, n.prefix.Unmasked(), desiredMask, n.prefix.mask)

	bestMatch := matches[0]
	for i := 1; i < len(matches); i++ {
		if matches[i].depth > bestMatch.depth {
			bestMatch = matches[i]
		}
	}

	return &Address{v4: bestMatch.path, mask: desiredMask}
}

func findSmallest(root *Tree, path uint, mask, depth int) []match {
	// some range has already been assigned
	if depth > mask {
		return nil
	}

	if root == nil {
		m := match{path, depth}
		return []match{m}
	}

	if root.Val != nil {
		return nil
	}

	return append(
		findSmallest(root.L, path, mask, depth+1),
		findSmallest(root.R, path|bitmask[depth], mask, depth+1)...,
	)
}

var sizes = flag.String("sizes", "20", "size of cidr range")
var project = flag.String("project", "", "gcloud project")
var region = flag.String("region", "us-central1", "gcloud region")
var network = flag.String("network", "", "network to search in")

func main() {
	flag.Parse()
	targetNetwork := *network
	project := *project
	region := *region
	sizes := *sizes

	xs := []int{}
	for _, s := range strings.Split(sizes, ",") {
		x, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		xs = append(xs, x)
	}

	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		panic(err)
	}
	containers, err := container.NewService(ctx)
	if err != nil {
		panic(err)
	}

	existingSubnets := []*Address{}

	parent := fmt.Sprintf("projects/%s/locations/-", project)
	clusters, err := containers.Projects.Locations.Clusters.List(parent).Do()
	if err != nil {
		panic(err)
	}

	for _, cluster := range clusters.Clusters {
		if cluster.Network != targetNetwork {
			continue
		}

		existingSubnets = append(existingSubnets, parseIP("clusters/"+cluster.Name, cluster.MasterIpv4CidrBlock))
	}

	req := computeService.Subnetworks.List(project, region)
	if err := req.Pages(ctx, func(page *compute.SubnetworkList) error {
		for _, subnetwork := range page.Items {
			shortname := networkShortName(subnetwork.Network)
			if shortname != targetNetwork {
				continue
			}
			existingSubnets = append(existingSubnets, parseIP("subnet/"+subnetwork.Name, subnetwork.IpCidrRange))
			for _, sec := range subnetwork.SecondaryIpRanges {
				existingSubnets = append(existingSubnets, parseIP("secondary/"+sec.RangeName, sec.IpCidrRange))
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}

	addressReq := computeService.Addresses.List(project, region)
	if err := addressReq.Pages(ctx, func(page *compute.AddressList) error {
		for _, address := range page.Items {
			if address.Network != targetNetwork {
				continue
			}
			existingSubnets = append(existingSubnets, parseIP("addresses/"+address.Name, address.Address))
		}
		return nil
	}); err != nil {
		panic(err)
	}

	network := &Network{
		prefix: parseIP("root", "10.0.0.0/8"),
	}

	for _, sub := range existingSubnets {
		network.Insert(sub)
	}

	for _, x := range xs {
		next := network.FindSmallest(x)
		next.name = "< new >"
		network.Insert(next)
	}
	network.Print()
}
