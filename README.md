# Traefik Headers Rules Plugin

This plugin for [Traefik](https://github.com/traefik/traefik) intercepts HTTP traffic, checks a specific header's value against a regular expression, and conditionally sets another header based on the match. It supports evaluating both **Request** headers (before they reach the backend) and **Response** headers (before they reach the client).

## Configuration

You can configure as many `requestRules` and `responseRules` as you need.

### Example

```yaml
http:
  middlewares:
    my-headers-rules:
      plugin:
        headers-rules:
          requestRules:
            # If the user agent contains "curl", set "X-Is-Curl: true"
            - checkHeader: "User-Agent"
              checkRegex: "curl/(.+)"
              setHeader: "X-Is-Curl"
              setValue: "true"
          responseRules:
            # Add a long Cache-Control to all successful Image responses
            - checkHeader: "Content-Type"
              checkRegex: "^image/(.+)$"
              setHeader: "Cache-Control"
              setValue: "public, max-age=31536000"
```

### Rule Properties

| Property      | Type     | Description                                                                  |
| ------------- | -------- | ---------------------------------------------------------------------------- |
| `checkHeader` | `string` | The header name to inspect                                                   |
| `checkRegex`  | `string` | The regular expression to match against the head's value                     |
| `checkMethod` | `string` | Specifically match an HTTP Method                                            |
| `checkPath`   | `string` | The regular expression to match against the request path                     |
| `checkStatus` | `int`    | Specify an exact response status code to match. Primarily for Response Rules |
| `setHeader`   | `string` | The header name to set if the conditions match                               |
| `setValue`    | `string` | The value to apply to the `setHeader`                                        |

_Note: All check properties act as a logical AND._

## Development

Run tests:

```bash
make test
```
