import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// The font's @font-face rules: upright only, every weight in one file. Vite
// copies the files it names into /assets (docs/design-brief.md §4).
import "@fontsource-variable/atkinson-hyperlegible-next/wght.css";
import "./index.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
