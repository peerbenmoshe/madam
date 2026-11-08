// SPIKE: the CSP log first, so it hears violations from the first render.
import "./spike/cspLog";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import GraphSpike from "./spike/GraphSpike.tsx";

// SPIKE: /spike shows the graph spike; every other path the app as before.
const isSpike = window.location.pathname.startsWith("/spike");

createRoot(document.getElementById("root")!).render(
  <StrictMode>{isSpike ? <GraphSpike /> : <App />}</StrictMode>,
);
