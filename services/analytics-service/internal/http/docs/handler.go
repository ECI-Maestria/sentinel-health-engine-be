package docs

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var specFS embed.FS
var specBytes []byte

func init() {
	b, err := specFS.ReadFile("openapi.yaml")
	if err != nil {
		panic("docs: failed to read embedded openapi.yaml: " + err.Error())
	}
	specBytes = b
}

func ServeSpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", specBytes)
}

func ServeUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerUIHTML)
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Sentinel Health Engine – Analytics API</title>
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 48 48'%3E%3Crect width='48' height='48' rx='10' fill='%233DAA7A'/%3E%3Ctext x='24' y='32' font-family='Arial' font-size='18' font-weight='bold' text-anchor='middle' fill='%23FFFFFF' letter-spacing='1'%3ESH%3C/text%3E%3C/svg%3E" />
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
  <style>html{box-sizing:border-box;overflow-y:scroll;}*,*:before,*:after{box-sizing:inherit;}body{margin:0;background:#fafafa;}.topbar{display:none;}#custom-header{background:#1a202c;color:#e2e8f0;padding:14px 24px;display:flex;align-items:center;gap:12px;font-family:sans-serif;}#custom-header h1{font-size:1.1rem;margin:0;font-weight:600;}#custom-header span{font-size:0.8rem;opacity:0.6;}</style>
</head>
<body>
  <div id="custom-header">
    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#68d391" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
    <h1>Sentinel Health Engine</h1><span>Analytics Service API</span>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({url: window.location.origin+'/openapi.yaml',dom_id:'#swagger-ui',deepLinking:true,presets:[SwaggerUIBundle.presets.apis,SwaggerUIStandalonePreset],layout:'StandaloneLayout',defaultModelsExpandDepth:1,displayRequestDuration:true,filter:true,tryItOutEnabled:true,persistAuthorization:true});
    };
  </script>
</body>
</html>`
