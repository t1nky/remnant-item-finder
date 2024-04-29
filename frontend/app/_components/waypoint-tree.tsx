interface WaypointTreeItemProps {
  children: React.ReactNode;
  nested?: boolean;
}

const WaypointTreeItem = ({ children, nested }: WaypointTreeItemProps) => {
  return (
    <div
      className={`${"relative"} ${
        nested
          ? "before:absolute before:top-0 before:bottom-1/2 before:-left-4 before:w-3 before:border-white before:content-[''] before:border-l-2 before:border-b-2 before:rounded-bl-md"
          : ""
      }`}
    >
      {children}
    </div>
  );
};

interface WaypointTreeProps {
  children: React.ReactNode;
}

const WaypointTree = ({ children }: WaypointTreeProps) => {
  return (
    <ul className="list-none m-0 p-0 pl-5 flex flex-col items-start text-sm gap-1">
      {children}
    </ul>
  );
};

WaypointTree.Item = WaypointTreeItem;

export default WaypointTree;
