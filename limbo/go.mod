module github.com/emuyx/cubedock/limbo

go 1.26.0

toolchain go1.27.4

require (
	github.com/Happy2018new/nemc-tan-lobby-solver v0.2.8
	github.com/df-mc/jsonc v1.0.5
	github.com/go-gl/mathgl v1.2.0
	github.com/google/uuid v1.6.0
	github.com/sandertv/gophertunnel v1.53.0
)

// Local copy with the NetEase resource-pack workaround; see PATCHES.md.
replace github.com/sandertv/gophertunnel => ./third_party/gophertunnel

// Reuse CubeDock's NetEase RakNet v8 client for the resource pack puller.
replace github.com/Happy2018new/nemc-tan-lobby-solver => ../vendor-src/nemc-tan-lobby-solver

require (
	github.com/coreos/go-oidc/v3 v3.17.0 // indirect
	github.com/df-mc/go-playfab v1.0.0 // indirect
	github.com/df-mc/go-xsapi v1.0.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/klauspost/compress v1.20.2 // indirect
	github.com/sandertv/go-raknet v1.14.3-0.20250305181847-6af3e95113d6 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)
