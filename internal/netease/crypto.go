package netease

import (
	"crypto/aes"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// md5Token returns the 16-byte key derived from the account token.
func md5Token(token string) []byte {
	sum := md5.Sum([]byte(token))
	return sum[:]
}

// aesECBEncrypt implements the PKCS#7 padded AES-ECB form used by the NetEase
// authentication envelope.
func aesECBEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 密钥无效: %w", err)
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	payload := append(append([]byte(nil), plaintext...), repeatByte(byte(padding), padding)...)
	out := make([]byte, len(payload))
	for offset := 0; offset < len(payload); offset += aes.BlockSize {
		block.Encrypt(out[offset:offset+aes.BlockSize], payload[offset:offset+aes.BlockSize])
	}
	return out, nil
}

func repeatByte(value byte, count int) []byte {
	out := make([]byte, count)
	for index := range out {
		out[index] = value
	}
	return out
}

// SelectTransferServer filters unusable entries and picks one at random.
func SelectTransferServer(servers []TransferServer, random io.Reader) (TransferServer, error) {
	available := make([]TransferServer, 0, len(servers))
	preferred := make([]TransferServer, 0, len(servers))
	for _, server := range servers {
		if server.IP == "" || server.SignalWebPort < 1 || server.SignalWebPort > 65535 || len(server.Ports) == 0 {
			continue
		}
		usable := true
		for _, port := range server.Ports {
			if port < 1 || port > 65535 {
				usable = false
				break
			}
		}
		if !usable {
			continue
		}
		available = append(available, server)
		// dockerB is the legacy room-control node type used by protocol 43;
		// dockerA nodes accept rooms but ignore room-side kick requests.
		if strings.EqualFold(server.ServerType, "dockerB") {
			preferred = append(preferred, server)
		}
	}
	if len(available) == 0 {
		return TransferServer{}, fmt.Errorf("没有可用的网易房间转发节点")
	}
	if len(preferred) > 0 {
		available = preferred
	}
	value, err := readRandomByte(random)
	if err != nil {
		return TransferServer{}, err
	}
	return available[int(value)%len(available)], nil
}

// DeriveRoomAuth builds the transfer credentials for one TanLobby room.
func DeriveRoomAuth(userID uint32, playerName, userToken string, server TransferServer, random io.Reader) (RoomAuth, error) {
	if len(server.Ports) == 0 {
		return RoomAuth{}, fmt.Errorf("转发节点没有可用端口")
	}
	encrypted := md5Token(userToken)
	raknetRand := make([]byte, 16)
	if _, err := io.ReadFull(random, raknetRand); err != nil {
		return RoomAuth{}, fmt.Errorf("生成 RakNet 随机数失败: %w", err)
	}
	raknetAESRand, err := aesECBEncrypt(raknetRand, encrypted)
	if err != nil {
		return RoomAuth{}, err
	}
	if len(raknetAESRand) < 16 {
		return RoomAuth{}, fmt.Errorf("RakNet AES 随机数长度无效")
	}
	raknetAESRand = raknetAESRand[:16]

	seed := make([]byte, 16)
	if _, err := io.ReadFull(random, seed); err != nil {
		return RoomAuth{}, fmt.Errorf("生成信令种子失败: %w", err)
	}
	ticket, err := aesECBEncrypt(seed, []byte(userToken))
	if err != nil {
		return RoomAuth{}, err
	}
	if len(ticket) < 16 {
		return RoomAuth{}, fmt.Errorf("信令票据长度无效")
	}
	ticket = ticket[:16]

	encryptKey := append(append([]byte(nil), encrypted...), raknetRand...)
	decryptKey := append(append([]byte(nil), raknetRand...), encrypted...)
	portValue, err := readRandomByte(random)
	if err != nil {
		return RoomAuth{}, err
	}
	port := server.Ports[int(portValue)%len(server.Ports)]

	auth := RoomAuth{
		UserID:           userID,
		PlayerName:       playerName,
		RaknetAddress:    net.JoinHostPort(server.IP, strconv.Itoa(port)),
		SignalingAddress: net.JoinHostPort(server.IP, strconv.Itoa(server.SignalWebPort)),
		RaknetRand:       raknetRand,
		RaknetAESRand:    raknetAESRand,
		SignalingSeed:    seed,
		SignalingTicket:  ticket,
		EncryptKey:       encryptKey,
		DecryptKey:       decryptKey,
	}
	if err := ValidateRoomAuth(auth); err != nil {
		return RoomAuth{}, err
	}
	return auth, nil
}

// ValidateRoomAuth enforces the fixed material sizes expected by room.Open.
func ValidateRoomAuth(auth RoomAuth) error {
	if len(auth.RaknetRand) != 16 {
		return fmt.Errorf("RakNet 随机数长度无效")
	}
	if len(auth.RaknetAESRand) < 16 {
		return fmt.Errorf("RakNet AES 随机数长度无效")
	}
	if len(auth.SignalingSeed) != 16 {
		return fmt.Errorf("信令种子长度无效")
	}
	if len(auth.SignalingTicket) < 16 {
		return fmt.Errorf("信令票据长度无效")
	}
	if len(auth.EncryptKey) != 32 || len(auth.DecryptKey) != 32 {
		return fmt.Errorf("房间加密密钥长度无效")
	}
	if _, _, err := net.SplitHostPort(auth.RaknetAddress); err != nil {
		return fmt.Errorf("RakNet 地址无效")
	}
	if _, _, err := net.SplitHostPort(auth.SignalingAddress); err != nil {
		return fmt.Errorf("信令地址无效")
	}
	return nil
}

func readRandomByte(random io.Reader) (byte, error) {
	var value [1]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return 0, fmt.Errorf("读取随机数失败: %w", err)
	}
	return value[0], nil
}
