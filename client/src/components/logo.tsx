import { Link } from "@tanstack/react-router";
import sift from "../assets/sift.png";

type LogoProps = {
  noText?: boolean;
};
export default function Logo({ noText }: LogoProps) {
  return (
    <Link className="gap-2 flex items-center" to="/">
      {/* TODO: put logo here */}
      <img src={sift} className="size-14" />
      {!noText && <p className="font-bold text-5xl">Sift</p>}
    </Link>
  );
}
