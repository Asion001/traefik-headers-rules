# Traefik Headers Rules Plugin

This plugin for [Traefik](https://github.com/traefik/traefik) intercepts HTTP traffic and allows you to dynamically set headers on your **Requests** (before they reach the backend) or **Responses** (before they reach the client) using a powerful boolean expression engine.

## Installation

Add the plugin to your Traefik static configuration:

```yaml
# traefik.yml
experimental:
  plugins:
    headers-rules:
      moduleName: "github.com/Asion001/traefik-headers-rules"
      version: "v0.1.0" # replace with the desired version
```

## Configuration

You can configure as many `requestRules` and `responseRules` as you need. Each rule evaluates an `expression`. If the expression evaluates to `true`, the `setHeader` and `setValue` actions are executed.

### Expressions DSL

The plugin uses a custom expression engine that supports logical operators (`&&`, `||`, `!`) and specific functions to inspect requests and responses:

- `Header("HeaderNameRegex", "RegexPattern")`: Checks if any header matching the name regex has a value that matches the pattern regex. (e.g., `Header("^X-.*", "^foo$")`)
- `Method("REGEX_METHOD")`: Checks if the HTTP request method matches the regex (e.g., `"^GET|POST$"`).
- `Path("RegexPattern")`: Checks if the request path matches the given regex.
- `Status("RegexStatusCode")`: Checks if the response status matches the regex (e.g., `"^2..$"` or `"404"`). Only applicable in `responseRules`.

### Example

```yaml
http:
  middlewares:
    my-headers-rules:
      plugin:
        headers-rules:
          requestRules:
            # If the method is POST and path starts with /api/v1/
            - expression: 'Path("^/api/v1/.*") && Method("POST")'
              setHeader: "X-Api-Call"
              setValue: "active"

            # If the user agent contains "curl"
            - expression: 'Header("User-Agent", "curl/(.+)")'
              setHeader: "X-Is-Curl"
              setValue: "true"

          responseRules:
            # Add a long Cache-Control to all successful Image responses
            - expression: 'Status("200") && Header("Content-Type", "^image/(.+)$")'
              setHeader: "Cache-Control"
              setValue: "public, max-age=31536000"
```

### Rule Properties

| Property     | Type     | Description                                                           |
| ------------ | -------- | --------------------------------------------------------------------- |
| `expression` | `string` | The boolean logic expression dictating when the header should be set. |
| `setHeader`  | `string` | The header name to set if the conditions in the expression are met.   |
| `setValue`   | `string` | The value to apply to the `setHeader`.                                |

## Development

Run tests:

```bash
make test
```
