package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1beta1"
	"os"
	"strconv"
	"strings"
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
	fmt.Println(name, ip)
	parts := strings.Split(ip, "/")
	if len(parts) != 2 {
		parts = []string{ip, "32"}
		//panic("unexpected address format " + ip)
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
			fmt.Println("existing overlap:", root.Val, a)
			return root
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

func (n *Network) FindSmallest(under *Address, desiredMask int) *Address {
	root := findExistingRoot(n.root, under, n.prefix.mask)
	if root == nil {
		return &Address{v4: under.v4, mask: desiredMask}
	}

	fmt.Printf("ROOT: %+v\n", root)
	//matches := findSmallest(n.root, n.prefix.Unmasked(), desiredMask, n.prefix.mask)
	//matches := findSmallest(root, under.Unmasked(), desiredMask, n.prefix.mask)
	//matches := findSmallest(root, root.Val.Unmasked(), desiredMask, root.Val.mask)
	matches := findSmallest(root, under.Unmasked(), desiredMask, under.mask)

	bestMatch := matches[0]
	for i := 1; i < len(matches); i++ {
		if matches[i].depth > bestMatch.depth {
			bestMatch = matches[i]
		}
	}

	return &Address{v4: bestMatch.path, mask: desiredMask}
}

func findExistingRoot(root *Tree, a *Address, depth int) *Tree {
	if root == nil {
		return nil
	}

	if depth == a.mask {
		return root
	}

	p := a.v4 & bitmask[depth]
	if p == 0 {
		return findExistingRoot(root.L, a, depth+1)
	}
	return findExistingRoot(root.R, a, depth+1)
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

// type Args struct {
// 	args []string
// 	j    int
// }
//
// func (s *Args) Next() bool {
// 	return s.j < len(s.args)
// }
//
// func (s *Args) Shift() string {
// 	r := s.args[s.j]
// 	s.j++
// 	return r
// }
//
// func (s *Args) peek() string {
// 	return s.args[s.j]
// }
//
// func (s *Args) parseRange() req {
// 	under := s.Shift()
//
// }
//
// func (s *Args) Parse(args []string) []req {
// 	for _, arg := range args {
// 		s.args = append(s.args, strings.Split(arg, "=")...)
// 	}
//
// 	ret := []req{}
// 	r := req{}
// 	for s.Next() {
// 		switch v := s.Shift(); v {
// 		case "--range":
//
// 		default:
// 		}
// 	}
// }

type req struct {
	cidr  string
	sizes []int
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

type Args struct {
	reqs    []req
	project string
	region  string
}

func parseargs() *Args {

	ret := &Args{region: "us-central1"}

	for _, x := range os.Args[1:] {
		fmt.Println(x)
		if strings.Index(x, "--range") == 0 {
			x := x[7:]
			parts := strings.Split(x, ":")
			sizes := []int{}
			for _, s := range strings.Split(parts[1], ",") {
				sizes = append(sizes, atoi(s))
			}

			ret.reqs = append(ret.reqs, req{
				cidr:  parts[0],
				sizes: sizes,
			})
		}

		if strings.Index(x, "--project") == 0 {
			ret.project = x[9:]
		}
	}

	return ret
	//args := os.Args[2:]
	//for _, arg := range os.Args[2:] {
	//	if strings.Index(arg, "--cidr") == 0 {

	//	}
	//}
}

// 192.168.10.0/24
// https://cloud.google.com/build/docs/private-pools/set-up-private-pool-to-use-in-vpc-network#setup-private-connection
func main() {
	//flag.Parse()
	//targetNetwork := *network
	//project := *project
	//TODO: need all regions
	//region := *region
	//sizes := *sizes

	args := parseargs()
	fmt.Println(args)
	project := args.project
	region := args.region

	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		panic(err)
	}
	containers, err := container.NewService(ctx)
	if err != nil {
		panic(err)
	}

	//existingSubnets := []*Address{}
	existingNetworks := map[string][]*Address{}
	networkRanges := map[string]string{}

	if err := computeService.Networks.List(project).Pages(ctx, func(page *compute.NetworkList) error {
		for _, network := range page.Items {
			networkRanges[network.Name] = network.IPv4Range
		}
		return nil
	}); err != nil {
		panic(err)
	}

	parent := fmt.Sprintf("projects/%s/locations/-", project)
	clusters, err := containers.Projects.Locations.Clusters.List(parent).Do()
	if err != nil {
		panic(err)
	}

	for _, cluster := range clusters.Clusters {
		existingNetworks[cluster.Network] = append(existingNetworks[cluster.Network], parseIP("clusters/"+cluster.Name, cluster.MasterIpv4CidrBlock))
	}

	req := computeService.Subnetworks.List(project, region)
	if err := req.Pages(ctx, func(page *compute.SubnetworkList) error {
		for _, subnetwork := range page.Items {
			shortname := networkShortName(subnetwork.Network)

			existingNetworks[shortname] = append(existingNetworks[shortname], parseIP("subnet/"+subnetwork.Name, subnetwork.IpCidrRange))
			for _, sec := range subnetwork.SecondaryIpRanges {
				existingNetworks[shortname] = append(existingNetworks[shortname], parseIP("secondary/"+sec.RangeName, sec.IpCidrRange))
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}

	addressReq := computeService.Addresses.List(project, region)
	if err := addressReq.Pages(ctx, func(page *compute.AddressList) error {
		for _, address := range page.Items {
			existingNetworks[address.Network] = append(existingNetworks[address.Network], parseIP("addresses/"+address.Name, address.Address))
		}
		return nil
	}); err != nil {
		panic(err)
	}

	fmt.Println(existingNetworks)
	fmt.Println(networkRanges)
	nw := &Network{
		prefix: parseIP("root", "0.0.0.0/0"),
	}
	for network, subnets := range existingNetworks {
		if len(network) == 0 {
			continue
		}

		if strings.Index(network, "example") >= 0 {
			continue
		}

		for _, sub := range subnets {
			nw.Insert(sub)
		}

	}
	//nw.Print()

	//for _, sub := range existingSubnets {
	//	network.Insert(sub)
	//}

	for _, req := range args.reqs {
		fmt.Println(req)
		under := parseIP("root", req.cidr)
		for _, x := range req.sizes {
			next := nw.FindSmallest(under, x)
			next.name = fmt.Sprintf("< new %s >", req.cidr)
			nw.Insert(next)
		}
	}
	//under := parseIP("target-network", targetNetwork)

	//xs := []int{}
	//for _, s := range strings.Split(sizes, ",") {
	//	x, err := strconv.Atoi(s)
	//	if err != nil {
	//		panic(err)
	//	}
	//	xs = append(xs, x)
	//}

	//for _, x := range xs {
	//	next := nw.FindSmallest(under, x)
	//	next.name = "< new >"
	//	nw.Insert(next)
	//}
	nw.Print()
}
