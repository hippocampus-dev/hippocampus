import { h } from "https://cdn.skypack.dev/preact@10.22.1";

export default class DefaultRenderer {
  render(resource, editable, setEditable) {
    return h(
      "div",
      {
        class:
          "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full overflow-hidden break-all",
      },
      [h("p", {}, "This resource is not supported.")],
    );
  }

  patch(resource, editable) {
    return {};
  }

  editable(resource) {
    return new Map();
  }
}
