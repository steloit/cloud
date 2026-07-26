import { Icon, type IconId } from "./icon";

export function Glyph({ id }: { id: IconId }) {
  return (
    <span className="glyph">
      <Icon id={id} />
    </span>
  );
}
