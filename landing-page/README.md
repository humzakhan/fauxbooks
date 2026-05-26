# fauxbooks-landing

Static landing page for fauxbooks, deployed to Cloudflare Pages.

Plain static HTML/CSS/JS with no build step. The `public/` directory is served as-is.

## Directory layout

```
landing-page/
  public/
    index.html        # main page
    styles.css        # styles
    interactions.js   # copy, balance count-up, hero terminal, file explorer
  package.json
  wrangler.toml
  .gitignore
  README.md
```

## Local preview

Install dependencies once:

```sh
npm install
```

Dev server with hot reload (auto-refreshes the browser on save):

```sh
npm run dev
# opens http://localhost:8788
```

Cloudflare-accurate preview with Wrangler (no hot reload):

```sh
npm run preview
# open http://localhost:8788
```

Fallback, plain static server with no extra deps:

```sh
npm run serve
```

## Deploy

Authenticate with Cloudflare (one-time):

```sh
wrangler login
```

Deploy to Cloudflare Pages:

```sh
npm run deploy
```
