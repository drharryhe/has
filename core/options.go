package core

type ServerOptions struct {
	Router        IRouter
	Plugins       []IPlugin
	AssetsManager IAssetManager
}

type APIGatewayOptions struct {
	ServerOptions

	Connectors  []IAPIConnector
	Middlewares []IAPIMiddleware
	Packers     []IAPIDataPacker
	I18n        IAPIi18n
}
