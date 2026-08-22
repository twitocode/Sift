import mole from "../assets/mole.png";

type LogoProps = {
  noText?: boolean;
};
export default function Logo({ noText }: LogoProps) {
  return (
    <span className="gap-2 flex items-center">
      {/* TODO: put logo here */}
      <img src={mole} className="size-15" />
      {!noText && <p className="font-bold text-5xl">Sift</p>}
    </span>
  );
}
