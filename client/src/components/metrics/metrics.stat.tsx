export function Stat({ label, value }: { label: string; value: string }) {
	return (
		<div className="flex flex-col gap-1 rounded-lg  bg-white/40 p-3">
			<span className="text-xs tracking-wide text-gray-500">{label}</span>
			<span className="text-lg font-bold text-gray-900">{value}</span>
		</div>
	);
}
