import { subprocessors } from '@/lib/legal/operator';

// The sub-processor list shared by the privacy policy and the DPA (report B09).
export function SubprocessorTable() {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-border text-foreground">
            <th className="py-2 pr-4 font-medium">Sub-processor</th>
            <th className="py-2 pr-4 font-medium">Purpose</th>
            <th className="py-2 font-medium">Location</th>
          </tr>
        </thead>
        <tbody>
          {subprocessors.map((s) => (
            <tr key={s.name} className="border-b border-border align-top">
              <td className="py-2 pr-4 text-foreground">{s.name}</td>
              <td className="py-2 pr-4">{s.purpose}</td>
              <td className="py-2">{s.location}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
