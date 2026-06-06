import "./style.css";

const app = document.querySelector<HTMLElement>("#app");

if (!app) {
  throw new Error("Missing #app element");
}

app.innerHTML = `
  <h1>Orphion Migaku Compatibility Harness</h1>
  <p>Variant: not selected</p>
`;
