const SITE_NAME = "Bank Soal";

export const DEFAULT_DESCRIPTION =
  "Latihan soal online gratis untuk siswa kelas 1–12. Paket soal pilihan ganda, benar/salah, isian, dan uraian lengkap dengan timer, nilai otomatis, dan pembahasan. Tanpa login, langsung dari HP atau laptop.";

type SeoOptions = {
  title: string;
  description?: string;
  path?: string;
  noindex?: boolean;
};

function upsertMeta(attr: "name" | "property", key: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(
    `meta[${attr}="${key}"]`
  );
  if (!el) {
    el = document.createElement("meta");
    el.setAttribute(attr, key);
    document.head.appendChild(el);
  }
  el.setAttribute("content", content);
}

function upsertCanonical(href: string) {
  let el = document.head.querySelector<HTMLLinkElement>(
    'link[rel="canonical"]'
  );
  if (!el) {
    el = document.createElement("link");
    el.setAttribute("rel", "canonical");
    document.head.appendChild(el);
  }
  el.setAttribute("href", href);
}

/** Perbarui title, meta description, robots, canonical, dan tag sosial per halaman. */
export function applySeo({ title, description, path, noindex }: SeoOptions) {
  const fullTitle = title.includes(SITE_NAME)
    ? title
    : `${title} — ${SITE_NAME}`;
  const desc = description || DEFAULT_DESCRIPTION;

  document.title = fullTitle;
  upsertMeta("name", "description", desc);
  upsertMeta("name", "robots", noindex ? "noindex, nofollow" : "index, follow");

  const url = `${window.location.origin}${path ?? window.location.pathname}`;
  upsertMeta("property", "og:title", fullTitle);
  upsertMeta("property", "og:description", desc);
  upsertMeta("property", "og:url", url);
  upsertMeta("name", "twitter:title", fullTitle);
  upsertMeta("name", "twitter:description", desc);
  upsertCanonical(url);
}