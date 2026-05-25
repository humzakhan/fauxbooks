# fauxbooks-landing

Static landing page for fauxbooks, deployed to Cloudflare Pages.

Plain static HTML/CSS/JS with no build step. The `public/` directory is served as-is.

## Directory layout

```
landing-page/
  public/
    index.html      # main page
    styles.css      # styles
    ascii-bg.js     # ascii background animation
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

Preview with Wrangler (matches Cloudflare Pages environment):

```sh
npm run dev
# open http://localhost:8788
```

Fallback — plain static server, no Wrangler required:

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
