package g79client

const (
	g79AndroidStep       = "695616851"
	g79AndroidStep2      = "2146985406"
	g79LibraryHash       = "815873bdd82cb7a4ce422da361f7d60d"
	g79SignatureHash     = "2b3e7ca013bb30a74d822579860c042b"
	g79BaseMCPHash       = ""
	g79AndroidMessageTag = "dashen_cloudgame"

	// 基岩互通版（网易发烧平台 Windows 启动器，sauth platform=pc）的 PE 认证参数。这些值与
	// g79client 的 Android 路径不同；2026-10-30 按官方 3.10.5.301486 客户端更新。
	// 抓包验证：官方 3.10 的 sign 匹配 PeAuthSign(message, 2, 8)，旧的 3/6 不匹配。
	interopEngineVersion = "3.10.5.301486"
	interopPatchVersion  = "3.10.5.301486"
	interopLibraryHash   = "43a8baa6ec5e3df458e707ebfcec65e5"
	interopPatchHash     = "330f7fa238110bf5ed88b5369e7f4504"
	interopSignHash      = "2b3e7ca013bb30a74d822579860c042b"
	interopSignSP        = 2
	interopSignTR        = 8
)

/*
3.6.5.281774
59cc563ed3c6430aff44500cd3f9e525
3.6.18.283127
22c622f4245a37ec3f1012a5325ddc25
2b3e7ca013bb30a74d822579860c042b
facb8049-0188-45b5-a0e5-8f149c0c1c91

3.6.5.281774
815873bdd82cb7a4ce422da361f7d60d
3.6.18.283127
22c622f4245a37ec3f1012a5325ddc25
2b3e7ca013bb30a74d822579860c042b
8888777a-03ff-4075-a3f3-e30363cc763b
*/

