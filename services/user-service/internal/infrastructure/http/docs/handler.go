// Package docs serves the OpenAPI specification and Swagger UI.
package docs

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var specFS embed.FS

// specBytes holds the raw YAML loaded at startup.
var specBytes []byte

func init() {
	b, err := specFS.ReadFile("openapi.yaml")
	if err != nil {
		panic("docs: failed to read embedded openapi.yaml: " + err.Error())
	}
	specBytes = b
}

// ServeSpec serves the raw OpenAPI YAML at GET /openapi.yaml.
func ServeSpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", specBytes)
}

// ServeUI serves the Swagger UI HTML page at GET /docs.
// The page loads the spec from /openapi.yaml on the same host,
// so it works identically in local development and in Azure.
func ServeUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerUIHTML)
}

// swaggerUIHTML is a self-contained Swagger UI page that fetches
// /openapi.yaml from the same origin (no hardcoded host).
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Sentinel Health Engine – API Docs</title>
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 48 48'%3E%3Crect width='48' height='48' rx='10' fill='%233DAA7A'/%3E%3Ctext x='24' y='32' font-family='Arial' font-size='18' font-weight='bold' text-anchor='middle' fill='%23FFFFFF' letter-spacing='1'%3ESH%3C/text%3E%3C/svg%3E" />
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }                /* hide default swagger top bar */
    #custom-header {
      background: #1a202c;
      color: #e2e8f0;
      padding: 14px 24px;
      display: flex;
      align-items: center;
      gap: 12px;
      font-family: sans-serif;
    }
    #custom-header h1 { font-size: 1.1rem; margin: 0; font-weight: 600; }
    #custom-header span { font-size: 0.8rem; opacity: 0.6; }
  </style>
</head>
<body>
  <div id="custom-header">
    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#68d391" stroke-width="2">
      <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
    </svg>
    <h1>Sentinel Health Engine</h1>
    <span>User Service API</span>
  </div>
  <div id="swagger-ui"></div>

  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function () {
      // Resolve spec URL relative to the current origin so the page works
      // on localhost:8080 AND on the Azure Container App URL without changes.
      const specUrl = window.location.origin + '/openapi.yaml';

      SwaggerUIBundle({
        url: specUrl,
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset,
        ],
        layout: 'StandaloneLayout',
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 2,
        displayRequestDuration: true,
        filter: true,
        tryItOutEnabled: true,
        persistAuthorization: true,
      });
    };
  </script>
</body>
</html>`
