// Package console renders the CubeDock startup banner and LAN endpoint list.
package console

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

const borderColor = "38;2;53;211;220"

// pixelFont is a tiny 5-row bitmap font for the console wordmark.
var pixelFont = map[byte][5]string{
	'C': {"01111", "10000", "10000", "10000", "01111"},
	'U': {"10001", "10001", "10001", "10001", "01110"},
	'B': {"11110", "10001", "11110", "10001", "11110"},
	'E': {"11111", "10000", "11110", "10000", "11111"},
	'D': {"11110", "10001", "10001", "10001", "11110"},
	'O': {"01110", "10001", "10001", "10001", "01110"},
	'K': {"10001", "10010", "11100", "10010", "10001"},
}

const wordmark = "CUBEDOCK"

// ColorEnabled reports whether ANSI colour is appropriate for stdout.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return supportsColor()
}

func ansi(enabled bool, text, code string) string {
	if !enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// Banner writes the wordmark and the build identity.
func Banner(w io.Writer, version, commit string) {
	if len(commit) > 7 {
		commit = commit[:7]
	}
	color := ColorEnabled()
	width := len(wordmark)*6 - 1 + 4
	edge := strings.Repeat("═", width)

	var out strings.Builder
	out.WriteString("\n")
	out.WriteString(ansi(color, "╔"+edge+"╗", borderColor))
	out.WriteString("\n")
	for row := 0; row < 5; row++ {
		out.WriteString(ansi(color, "║  ", borderColor))
		for index := 0; index < len(wordmark); index++ {
			bits := pixelFont[wordmark[index]][row]
			pixels := strings.Map(func(r rune) rune {
				if r == '1' {
					return '█'
				}
				return ' '
			}, bits)
			out.WriteString(ansi(color, pixels, fmt.Sprintf("38;2;%d;%d;%d", 96-index*5, 150+index*9, 220)))
			if index+1 < len(wordmark) {
				out.WriteByte(' ')
			}
		}
		out.WriteString(ansi(color, "  ║", borderColor))
		out.WriteByte('\n')
	}
	out.WriteString(ansi(color, "╚"+edge+"╝", borderColor))
	out.WriteByte('\n')
	fmt.Fprintf(&out, "  %s %s  ·  %s  ·  %s/%s\n",
		ansi(color, "CubeDock 方块码头", "1;36"), version, commit, runtime.GOOS, runtime.GOARCH)
	out.WriteString("  网易本地联机 → 网易协议上游 · RakNet v8 直通\n\n")
	_, _ = io.WriteString(w, out.String())
}

// IPv4Addresses returns active, non-loopback IPv4 addresses. Duplicates and
// link-local/unspecified addresses are skipped.
func IPv4Addresses() []net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	unique := make(map[string]net.IP)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, ok := usableIPv4(addr)
			if ok {
				unique[ip.String()] = ip
			}
		}
	}
	result := make([]net.IP, 0, len(unique))
	for _, ip := range unique {
		result = append(result, ip)
	}
	slices.SortFunc(result, func(left, right net.IP) int {
		return strings.Compare(left.String(), right.String())
	})
	return result
}

func usableIPv4(addr net.Addr) (net.IP, bool) {
	ip, _, err := net.ParseCIDR(addr.String())
	if err != nil {
		return nil, false
	}
	ip = ip.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return nil, false
	}
	return ip, true
}

// Hosts builds the Host-header allowlist for the console port.
func Hosts(port int, addresses []net.IP) map[string]bool {
	portText := strconv.Itoa(port)
	allowed := make(map[string]bool, len(addresses)+3)
	for _, host := range []string{"127.0.0.1", "localhost", "0.0.0.0"} {
		allowed[net.JoinHostPort(host, portText)] = true
	}
	for _, ip := range addresses {
		allowed[net.JoinHostPort(ip.String(), portText)] = true
	}
	return allowed
}

// URLs prints the local and LAN console endpoints.
func URLs(w io.Writer, port int, addresses []net.IP) {
	portText := strconv.Itoa(port)
	fmt.Fprintf(w, "CubeDock 控制台监听：0.0.0.0:%s\n", portText)
	fmt.Fprintf(w, "本机访问：http://127.0.0.1:%s/\n", portText)
	for _, ip := range addresses {
		fmt.Fprintf(w, "局域网访问：http://%s/\n", net.JoinHostPort(ip.String(), portText))
	}
	if len(addresses) == 0 {
		fmt.Fprintln(w, "未发现可用的局域网 IPv4 地址，请检查网络连接。")
	}
}
