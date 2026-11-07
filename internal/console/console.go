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
	"unicode"
)

const (
	amberCode   = "38;2;245;158;11"
	emeraldCode = "38;2;52;211;153"
	boldAmber   = "1;38;2;245;158;11"
)

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

// Banner writes a compact startup card with the build identity.
func Banner(w io.Writer, version, commit string) {
	if len(commit) > 7 {
		commit = commit[:7]
	}
	color := ColorEnabled()
	const cardWidth = 54
	title := padDisplay("CUBEDOCK  ▸  方块码头", cardWidth-4)
	subtitle := padDisplay("网易本地联机  →  网易协议上游  ·  RakNet v8 直通", cardWidth-4)
	edge := strings.Repeat("━", cardWidth)

	var out strings.Builder
	out.WriteByte('\n')
	out.WriteString(ansi(color, "┏"+edge+"┓", amberCode))
	out.WriteByte('\n')
	out.WriteString(ansi(color, "┃  ", amberCode))
	out.WriteString(ansi(color, title, boldAmber))
	out.WriteString(ansi(color, "  ┃", amberCode))
	out.WriteByte('\n')
	out.WriteString(ansi(color, "┃  ", amberCode))
	out.WriteString(ansi(color, subtitle, emeraldCode))
	out.WriteString(ansi(color, "  ┃", amberCode))
	out.WriteByte('\n')
	out.WriteString(ansi(color, "┗"+edge+"┛", amberCode))
	out.WriteByte('\n')
	fmt.Fprintf(&out, "  %s %s  ·  %s  ·  %s/%s\n\n",
		ansi(color, "CubeDock 方块码头", boldAmber), version, commit, runtime.GOOS, runtime.GOARCH)
	_, _ = io.WriteString(w, out.String())
}

func padDisplay(text string, width int) string {
	if missing := width - displayWidth(text); missing > 0 {
		return text + strings.Repeat(" ", missing)
	}
	return text
}

func displayWidth(text string) int {
	width := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || (r >= 0x3000 && r <= 0x9fff) {
			width += 2
		} else {
			width++
		}
	}
	return width
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
