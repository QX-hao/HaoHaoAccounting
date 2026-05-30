# Mobile API Client

React Native API helper layer.

## Responsibilities

- Persist bearer tokens in AsyncStorage.
- Attach bearer tokens to authenticated requests.
- Parse backend error payloads into thrown `Error` values.

Feature folders should expose business-named API functions on top of this client.
