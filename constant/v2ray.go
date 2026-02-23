package constant

const (
	V2RayTransportTypeHTTP        = "http"
	V2RayTransportTypeWebsocket   = "ws"
	V2RayTransportTypeQUIC        = "quic"
	V2RayTransportTypeGRPC        = "grpc"
	V2RayTransportTypeHTTPUpgrade = "httpupgrade"
	V2RayTransportTypeKCP         = "kcp"
	V2RayTransportTypeMKCP        = "mkcp"
	V2RayTransportTypeXHTTP       = "xhttp"
	V2RayTransportTypeSplitHTTP   = "splithttp" // alias for xhttp (Xray compat)
)
