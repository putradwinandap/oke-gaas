# TypeScript MVP integration example

This example exercises the complete Project-scoped MVP flow through the SDK:

1. register a Player
2. create an exact-event XP Rule
3. track one Event
4. read the resulting Player State

It intentionally does not create a Project. Project provisioning requires the operator/admin key, while the integration SDK is scoped to a single Project API key so applications do not mix operator credentials into normal runtime code.

Build the SDK first:

```bash
cd sdk/typescript
npm install
npm run build
cd ../..
```

Then run the example against a running Oke Gaas server:

```bash
OKE_GAAS_PROJECT_ID=proj_xxx \
OKE_GAAS_API_KEY=your-project-key \
node examples/typescript-mvp/index.mjs
```

Set `OKE_GAAS_BASE_URL` when the server is not at `http://localhost:8080`.

Each run uses unique Player, Rule event type, and Event identifiers so rerunning the example does not accidentally stack an old matching rule or collide with an old Event.
