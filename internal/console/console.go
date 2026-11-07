// Package console prints startup information for the CubeDock process.
package console

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// ColorEnabled reports whether ANSI colour should be used on stdout.
func ColorEnabled() bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("TERM") == "dumb" {
		return false
	}
	return enableColor()
}

// Banner prints the CubeDock wordmark and build identity.
func Banner(w io.Writer, version, commit string) {
	font := map[rune][]string{
		'C': {"01111", "10000", "10000", "10000", "01111"},
		'U': {"10001", "10001", "10001", "10001", "01110"},
		'B': {"11110", "10001", "11110", "10001", "11110"},
		'E': {"11111", "10000", "11110", "10000", "11111"},
		'D': {"11110", "10001", "10001", "10001", "11110"},
		'O': {"01110", "10001", "10001", "10001", "01110"},
		'K': {"10001", "10010", "11100", "10010", "10001"},
	}
	const title = "CUBEDOCK"
	color := ColorEnabled()
	paint := func(text, code string) string {
		if color {
			return "\x1b[" + code + "m" + text + "\x1b[0m"
		}
		return text
	}
	width := len(title)*6 - 1 + 4
	fmt.Fprintln(w)
	fmt.Fprintln(w, paint("╔"+strings.Repeat("═", width)+"╗", "38;2;53;211;220"))
	for row := 0; row < 5; row++ {
		fmt.Fprint(w, paint("║  ", "38;2;53;211;220"))
		for index, letter := range title {
			pixels := strings.NewReplacer("1", "█", "0", " ").Replace(font[letter][row])
			code := fmt.Sprintf("38;2;%d;%d;%d", 96-index*5, 150+index*9, 220)
			fmt.Fprint(w, paint(pixels, code))
			if index < len(title)-1 {
				fmt.Fprint(w, " ")
			}
		}
		fmt.Fprintln(w, paint("  ║", "38;2;53;211;220"))
	}
	fmt.Fprintln(w, paint("╚"+strings.Repeat("═", width)+"╝", "38;2;53;211;220"))
	if len(commit) > 7 {
		commit = commit[:7]
	}
	fmt.Fprintf(w, "  %s %s  ·  %s  ·  %s/%s\n",
		paint("CubeDock 方块码头", "1;36"), version, commit, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintln(w, "  网易本地联机 → 网易协议上游 · RakNet v8 直通")
	fmt.Fprintln(w)
}

// IPv4Addresses lists every active non-loopback IPv4 address so LAN and VPN
// consoles are both reachable.
func IPv4Addresses() []net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var addresses []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			if !seen[ip.String()] {
				seen[ip.String()] = true
				addresses = append(addresses, ip.To4())
			}
		}
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].String() < addresses[j].String() })
	return addresses
}

// Hosts is the Host-header allowlist. It keeps DNS-rebinding protection while
// permitting LAN access from the addresses printed at startup.
func Hosts(port int, addresses []net.IP) map[string]bool {
	allowed := map[string]bool{}
	for _, host := range []string{"127.0.0.1", "localhost", "0.0.0.0"} {
		allowed[net.JoinHostPort(host, strconv.Itoa(port))] = true
	}
	for _, ip := range addresses {
		allowed[net.JoinHostPort(ip.String(), strconv.Itoa(port))] = true
	}
	return allowed
}

// URLs prints the console endpoints.
func URLs(w io.Writer, port int, addresses []net.IP) {
	fmt.Fprintf(w, "CubeDock 控制台监听：0.0.0.0:%d\n", port)
	fmt.Fprintf(w, "本机访问：http://127.0.0.1:%d/\n", port)
	for _, ip := range addresses {
		fmt.Fprintf(w, "局域网访问：http://%s/\n", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
	}
	if len(addresses) == 0 {
		fmt.Fprintln(w, "未发现可用的局域网 IPv4 地址，请检查网络连接。")
	}
}
