import type { FC, CSSProperties } from 'react';
import { HugeiconsIcon } from '@hugeicons/react';
import {
  Mic01Icon as _Mic01Icon,
  Activity01Icon as _Activity01Icon,
  Alert01Icon as _Alert01Icon,
  AlertCircleIcon as _AlertCircleIcon,
  ArchiveIcon as _ArchiveIcon,
  ArchiveRestoreIcon as _ArchiveRestoreIcon,
  ArrowLeftRightIcon as _ArrowLeftRightIcon,
  TextAlignCenterIcon as _TextAlignCenterIcon,
  TextAlignLeftIcon as _TextAlignLeftIcon,
  TextAlignRightIcon as _TextAlignRightIcon,
  ArrowDown02Icon as _ArrowDown02Icon,
  ArrowUpDoubleIcon as _ArrowUpDoubleIcon,
  ArrowUpRight01Icon as _ArrowUpRight01Icon,
  AtIcon as _AtIcon,
  ArrowExpandIcon as _ArrowExpandIcon,
  ArrowLeft02Icon as _ArrowLeft02Icon,
  ArrowReloadHorizontalIcon as _ArrowReloadHorizontalIcon,
  ArrowRight02Icon as _ArrowRight02Icon,
  ArrowShrinkIcon as _ArrowShrinkIcon,
  ArrowTurnBackwardIcon as _ArrowTurnBackwardIcon,
  ArrowTurnDownIcon as _ArrowTurnDownIcon,
  ArrowTurnForwardIcon as _ArrowTurnForwardIcon,
  ArrowUp02Icon as _ArrowUp02Icon,
  AttachmentIcon as _AttachmentIcon,
  Award01Icon as _Award01Icon,
  Bookmark01Icon as _Bookmark01Icon,
  BookOpen01Icon as _BookOpen01Icon,
  BotIcon as _BotIcon,
  Bug01Icon as _Bug01Icon,
  Briefcase01Icon as _Briefcase01Icon,
  BulbIcon as _BulbIcon,
  Building03Icon as _Building03Icon,
  Calendar01Icon as _Calendar01Icon,
  Calendar03Icon as _Calendar03Icon,
  Camera01Icon as _Camera01Icon,
  Cancel01Icon as _Cancel01Icon,
  CollapseIcon as _CollapseIcon,
  ChartGanttIcon as _ChartGanttIcon,
  ChromeIcon as _ChromeIcon,
  ClipboardIcon as _ClipboardIcon,
  Clock03Icon as _Clock03Icon,
  CancelCircleIcon as _CancelCircleIcon,
  ChartColumnIcon as _ChartColumnIcon,
  ChartIncreaseIcon as _ChartIncreaseIcon,
  CheckListIcon as _CheckListIcon,
  CheckmarkCircle02Icon as _CheckmarkCircle02Icon,
  CheckmarkSquare02Icon as _CheckmarkSquare02Icon,
  CircleIcon as _CircleIcon,
  SquareIcon as _SquareIcon,
  HighlighterIcon as _HighlighterIcon,
  PenTool02Icon as _PenTool02Icon,
  CursorPointer01Icon as _CursorPointer01Icon,
  TextIcon as _TextIcon,
  ArrowMoveUpRightIcon as _ArrowMoveUpRightIcon,
  BubbleChatIcon as _BubbleChatIcon,
  Clock01Icon as _Clock01Icon,
  Clock02Icon as _Clock02Icon,
  CodeIcon as _CodeIcon,
  Comment01Icon as _Comment01Icon,
  ColumnsThreeCogIcon as _ColumnsThreeCogIcon,
  ComputerIcon as _ComputerIcon,
  Copy01Icon as _Copy01Icon,
  CursorTextIcon as _CursorTextIcon,
  DashboardSpeed01Icon as _DashboardSpeed01Icon,
  DashedLineCircleIcon as _DashedLineCircleIcon,
  DatabaseIcon as _DatabaseIcon,
  Delete01Icon as _Delete01Icon,
  DollarCircleIcon as _DollarCircleIcon,
  Download04Icon as _Download04Icon,
  DragDropVerticalIcon as _DragDropVerticalIcon,
  EraserIcon as _EraserIcon,
  ExpandIcon as _ExpandIcon,
  FavouriteIcon as _FavouriteIcon,
  File01Icon as _File01Icon,
  FileCodeIcon as _FileCodeIcon,
  FileDownIcon as _FileDownIcon,
  FileImportIcon as _FileImportIcon,
  FileSearchIcon as _FileSearchIcon,
  FileUpIcon as _FileUpIcon,
  FilterHorizontalIcon as _FilterHorizontalIcon,
  FilterIcon as _FilterIcon,
  FloppyDiskIcon as _FloppyDiskIcon,
  Folder01Icon as _Folder01Icon,
  FolderInputIcon as _FolderInputIcon,
  FolderKanbanIcon as _FolderKanbanIcon,
  FolderOpenIcon as _FolderOpenIcon,
  Forward01Icon as _Forward01Icon,
  GitBranchIcon as _GitBranchIcon,
  GitCommitIcon as _GitCommitIcon,
  GitPullRequestIcon as _GitPullRequestIcon,
  Globe02Icon as _Globe02Icon,
  GlobeIcon as _GlobeIcon,
  HashtagIcon as _HashtagIcon,
  Heading02Icon as _Heading02Icon,
  Heading03Icon as _Heading03Icon,
  Heading04Icon as _Heading04Icon,
  HeadphonesIcon as _HeadphonesIcon,
  HelpCircleIcon as _HelpCircleIcon,
  HexagonIcon as _HexagonIcon,
  HierarchyIcon as _HierarchyIcon,
  Image01Icon as _Image01Icon,
  InboxIcon as _InboxIcon,
  InformationCircleIcon as _InformationCircleIcon,
  KanbanIcon as _KanbanIcon,
  Key01Icon as _Key01Icon,
  LanguageCircleIcon as _LanguageCircleIcon,
  LaptopIcon as _LaptopIcon,
  Layers01Icon as _Layers01Icon,
  LayoutGridIcon as _LayoutGridIcon,
  LayoutTable01Icon as _LayoutTable01Icon,
  LayoutTwoColumnIcon as _LayoutTwoColumnIcon,
  LeftToRightListBulletIcon as _LeftToRightListBulletIcon,
  LeftToRightListNumberIcon as _LeftToRightListNumberIcon,
  LifebuoyIcon as _LifebuoyIcon,
  Link01Icon as _Link01Icon,
  LinkSquare01Icon as _LinkSquare01Icon,
  LockIcon as _LockIcon,
  LockKeyIcon as _LockKeyIcon,
  Logout01Icon as _Logout01Icon,
  MagicWand01Icon as _MagicWand01Icon,
  Mail01Icon as _Mail01Icon,
  MailAdd01Icon as _MailAdd01Icon,
  MailOpenIcon as _MailOpenIcon,
  MapPinIcon as _MapPinIcon,
  Maximize01Icon as _Maximize01Icon,
  Menu01Icon as _Menu01Icon,
  MinusSignIcon as _MinusSignIcon,
  Message01Icon as _Message01Icon,
  MessagePreview01Icon as _MessagePreview01Icon,
  MessageCircleReplyIcon as _MessageCircleReplyIcon,
  MailReply01Icon as _MailReply01Icon,
  Minimize01Icon as _Minimize01Icon,
  Moon02Icon as _Moon02Icon,
  MoreHorizontalIcon as _MoreHorizontalIcon,
  MoreVerticalIcon as _MoreVerticalIcon,
  Notification02Icon as _Notification02Icon,
  NotificationBubbleIcon as _NotificationBubbleIcon,
  NotificationOff02Icon as _NotificationOff02Icon,
  OctagonIcon as _OctagonIcon,
  OctagonXIcon as _OctagonXIcon,
  PaintBoardIcon as _PaintBoardIcon,
  PinIcon as _PinIcon,
  PinOffIcon as _PinOffIcon,
  PauseIcon as _PauseIcon,
  PencilEdit01Icon as _PencilEdit01Icon,
  PencilEdit02Icon as _PencilEdit02Icon,
  PlayCircleIcon as _PlayCircleIcon,
  PlayIcon as _PlayIcon,
  PlusSignCircleIcon as _PlusSignCircleIcon,
  PlusSignIcon as _PlusSignIcon,
  QuoteDownIcon as _QuoteDownIcon,
  RadioIcon as _RadioIcon,
  RecordIcon as _RecordIcon,
  ReplaceIcon as _ReplaceIcon,
  RotateLeft01Icon as _RotateLeft01Icon,
  Search01Icon as _Search01Icon,
  SecurityCheckIcon as _SecurityCheckIcon,
  SentIcon as _SentIcon,
  Setting06Icon as _Setting06Icon,
  Setting07Icon as _Setting07Icon,
  Settings02Icon as _Settings02Icon,
  Shield01Icon as _Shield01Icon,
  Shield02Icon as _Shield02Icon,
  SignalFull01Icon as _SignalFull01Icon,
  SignalLow01Icon as _SignalLow01Icon,
  SignalMedium01Icon as _SignalMedium01Icon,
  SmartPhone01Icon as _SmartPhone01Icon,
  SmileIcon as _SmileIcon,
  SmilePlusIcon as _SmilePlusIcon,
  SourceCodeIcon as _SourceCodeIcon,
  SparklesIcon as _SparklesIcon,
  AiMagicIcon as _AiMagicIcon,
  AiNetworkIcon as _AiNetworkIcon,
  CloudServerIcon as _CloudServerIcon,
  Unlink01Icon as _Unlink01Icon,
  SquareUnlock01Icon as _SquareUnlock01Icon,
  StopIcon as _StopIcon,
  StarIcon as _StarIcon,
  StickyNote01Icon as _StickyNote01Icon,
  Sun01Icon as _Sun01Icon,
  Table01Icon as _Table01Icon,
  Tablet01Icon as _Tablet01Icon,
  Tag01Icon as _Tag01Icon,
  Target01Icon as _Target01Icon,
  Target02Icon as _Target02Icon,
  TelephoneIcon as _TelephoneIcon,
  TextBoldIcon as _TextBoldIcon,
  TextItalicIcon as _TextItalicIcon,
  TextStrikethroughIcon as _TextStrikethroughIcon,
  TextUnderlineIcon as _TextUnderlineIcon,
  TerminalIcon as _TerminalIcon,
  Tick01Icon as _Tick01Icon,
  TickDouble01Icon as _TickDouble01Icon,
  Timer01Icon as _Timer01Icon,
  UndoIcon as _UndoIcon,
  Upload01Icon as _Upload01Icon,
  UserAdd01Icon as _UserAdd01Icon,
  UserCheck01Icon as _UserCheck01Icon,
  UserGroupIcon as _UserGroupIcon,
  UserIcon as _UserIcon,
  UserRemove01Icon as _UserRemove01Icon,
  ViewIcon as _ViewIcon,
  ViewOffIcon as _ViewOffIcon,
  WorkflowSquare01Icon as _WorkflowSquare01Icon,
  Wrench01Icon as _Wrench01Icon,
  ZapIcon as _ZapIcon,
  ZoomInAreaIcon as _ZoomInAreaIcon,
  ZoomOutAreaIcon as _ZoomOutAreaIcon,
} from '@hugeicons/core-free-icons';

export type IconComponent = FC<{ className?: string; style?: CSSProperties }>;

type HugeIconData = Parameters<typeof HugeiconsIcon>[0]['icon'];

function hi(icon: HugeIconData): IconComponent {
  const C = ({ className, style }: { className?: string; style?: CSSProperties }) => (
    <HugeiconsIcon icon={icon} className={className} style={style} strokeWidth={2} />
  );
  return C;
}

export const ArchiveRestoreIcon = hi(_ArchiveRestoreIcon);
export const ArrowLeftRightIcon = hi(_ArrowLeftRightIcon);
export const ArrowUpDoubleIcon = hi(_ArrowUpDoubleIcon);
export const ArrowUpRight01Icon = hi(_ArrowUpRight01Icon);
export const AtIcon = hi(_AtIcon);
export const Bookmark01Icon = hi(_Bookmark01Icon);
export const Bug01Icon = hi(_Bug01Icon);
export const ChartGanttIcon = hi(_ChartGanttIcon);
export const ChromeIcon = hi(_ChromeIcon);
export const ClipboardIcon = hi(_ClipboardIcon);
export const Clock03Icon = hi(_Clock03Icon);
export const DashedLineCircleIcon = hi(_DashedLineCircleIcon);
export const EraserIcon = hi(_EraserIcon);
export const ExpandIcon = hi(_ExpandIcon);
export const FileDownIcon = hi(_FileDownIcon);
export const FileSearchIcon = hi(_FileSearchIcon);
export const FileUpIcon = hi(_FileUpIcon);
export const FolderInputIcon = hi(_FolderInputIcon);
export const FolderKanbanIcon = hi(_FolderKanbanIcon);
export const HashtagIcon = hi(_HashtagIcon);
export const Heading02Icon = hi(_Heading02Icon);
export const Heading03Icon = hi(_Heading03Icon);
export const Heading04Icon = hi(_Heading04Icon);
export const HeadphonesIcon = hi(_HeadphonesIcon);
export const HexagonIcon = hi(_HexagonIcon);
export const LanguageCircleIcon = hi(_LanguageCircleIcon);
export const LaptopIcon = hi(_LaptopIcon);
export const Layers01Icon = hi(_Layers01Icon);
export const LockIcon = hi(_LockIcon);
export const LockKeyIcon = hi(_LockKeyIcon);
export const MinusSignIcon = hi(_MinusSignIcon);
export const OctagonIcon = hi(_OctagonIcon);
export const OctagonXIcon = hi(_OctagonXIcon);
export const PinIcon = hi(_PinIcon);
export const PinOffIcon = hi(_PinOffIcon);
export const QuoteDownIcon = hi(_QuoteDownIcon);
export const RadioIcon = hi(_RadioIcon);
export const RecordIcon = hi(_RecordIcon);
export const SignalFull01Icon = hi(_SignalFull01Icon);
export const SignalLow01Icon = hi(_SignalLow01Icon);
export const SignalMedium01Icon = hi(_SignalMedium01Icon);
export const SmartPhone01Icon = hi(_SmartPhone01Icon);
export const SquareUnlock01Icon = hi(_SquareUnlock01Icon);
export const StopIcon = hi(_StopIcon);
export const Table01Icon = hi(_Table01Icon);
export const Tablet01Icon = hi(_Tablet01Icon);
export const Target02Icon = hi(_Target02Icon);
export const TextBoldIcon = hi(_TextBoldIcon);
export const TextItalicIcon = hi(_TextItalicIcon);
export const TextStrikethroughIcon = hi(_TextStrikethroughIcon);
export const TextUnderlineIcon = hi(_TextUnderlineIcon);
export const UndoIcon = hi(_UndoIcon);
export const UserCheck01Icon = hi(_UserCheck01Icon);
export const UserRemove01Icon = hi(_UserRemove01Icon);
export const Wrench01Icon = hi(_Wrench01Icon);

export const Activity01Icon = hi(_Activity01Icon);
export const Alert01Icon = hi(_Alert01Icon);
export const AlertCircleIcon = hi(_AlertCircleIcon);
export const ArchiveIcon = hi(_ArchiveIcon);
export const TextAlignCenterIcon = hi(_TextAlignCenterIcon);
export const TextAlignLeftIcon = hi(_TextAlignLeftIcon);
export const TextAlignRightIcon = hi(_TextAlignRightIcon);
export const ArrowDown01Icon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M6 9L12 15L18 9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);
export const ArrowDown02Icon = hi(_ArrowDown02Icon);
export const ArrowExpandIcon = hi(_ArrowExpandIcon);
export const ArrowLeft01Icon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M15 6L9 12L15 18" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);
export const ArrowLeft02Icon = hi(_ArrowLeft02Icon);
export const ArrowReloadHorizontalIcon = hi(_ArrowReloadHorizontalIcon);
export const ArrowRight01Icon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M9 6L15 12L9 18" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);
export const ArrowRight02Icon = hi(_ArrowRight02Icon);
export const ArrowShrinkIcon = hi(_ArrowShrinkIcon);
export const ArrowTurnBackwardIcon = hi(_ArrowTurnBackwardIcon);
export const ArrowTurnDownIcon = hi(_ArrowTurnDownIcon);
export const ArrowTurnForwardIcon = hi(_ArrowTurnForwardIcon);
export const ArrowUp01Icon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M6 15L12 9L18 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);
export const ArrowUp02Icon = hi(_ArrowUp02Icon);
export const ArrowUpDownIcon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M7 15L12 20L17 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M7 9L12 4L17 9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);
export const AttachmentIcon = hi(_AttachmentIcon);
export const Award01Icon = hi(_Award01Icon);
export const BookOpen01Icon = hi(_BookOpen01Icon);
export const BotIcon = hi(_BotIcon);
export const Briefcase01Icon = hi(_Briefcase01Icon);
export const BulbIcon = hi(_BulbIcon);
export const Building03Icon = hi(_Building03Icon);
export const Calendar01Icon = hi(_Calendar01Icon);
export const Calendar03Icon = hi(_Calendar03Icon);
export const Camera01Icon = hi(_Camera01Icon);
export const Cancel01Icon = hi(_Cancel01Icon);
export const CollapseIcon = hi(_CollapseIcon);
export const CancelCircleIcon = hi(_CancelCircleIcon);
export const ChartColumnIcon = hi(_ChartColumnIcon);
export const ChartIncreaseIcon = hi(_ChartIncreaseIcon);
export const CheckListIcon = hi(_CheckListIcon);
export const CheckmarkCircle02Icon = hi(_CheckmarkCircle02Icon);
export const CheckmarkSquare02Icon = hi(_CheckmarkSquare02Icon);
export const CircleIcon = hi(_CircleIcon);
export const SquareIcon = hi(_SquareIcon);
export const HighlighterIcon = hi(_HighlighterIcon);
export const PenTool02Icon = hi(_PenTool02Icon);
export const CursorPointer01Icon = hi(_CursorPointer01Icon);
export const TextIcon = hi(_TextIcon);
export const ArrowMoveUpRightIcon = hi(_ArrowMoveUpRightIcon);
export const BubbleChatIcon = hi(_BubbleChatIcon);
export const Clock01Icon = hi(_Clock01Icon);
export const Clock02Icon = hi(_Clock02Icon);
export const CodeIcon = hi(_CodeIcon);
export const Comment01Icon = hi(_Comment01Icon);
export const ColumnsThreeCogIcon = hi(_ColumnsThreeCogIcon);
export const ComputerIcon = hi(_ComputerIcon);
export const Copy01Icon = hi(_Copy01Icon);
export const CursorTextIcon = hi(_CursorTextIcon);
export const DashboardSpeed01Icon = hi(_DashboardSpeed01Icon);
export const DatabaseIcon = hi(_DatabaseIcon);
export const Delete01Icon = hi(_Delete01Icon);
export const DollarCircleIcon = hi(_DollarCircleIcon);
export const Download04Icon = hi(_Download04Icon);
export const DragDropVerticalIcon = hi(_DragDropVerticalIcon);
export const FavouriteIcon = hi(_FavouriteIcon);
export const File01Icon = hi(_File01Icon);
export const FileCodeIcon = hi(_FileCodeIcon);
export const FileImportIcon = hi(_FileImportIcon);
export const FilterHorizontalIcon = hi(_FilterHorizontalIcon);
export const FilterIcon = hi(_FilterIcon);
export const FloppyDiskIcon = hi(_FloppyDiskIcon);
export const Folder01Icon = hi(_Folder01Icon);
export const FolderOpenIcon = hi(_FolderOpenIcon);
export const Forward01Icon = hi(_Forward01Icon);
export const GitBranchIcon = hi(_GitBranchIcon);
export const GitCommitIcon = hi(_GitCommitIcon);
export const GitPullRequestIcon = hi(_GitPullRequestIcon);
export const Globe02Icon = hi(_Globe02Icon);
export const GlobeIcon = hi(_GlobeIcon);
export const HelpCircleIcon = hi(_HelpCircleIcon);
export const HierarchyIcon = hi(_HierarchyIcon);
export const Image01Icon = hi(_Image01Icon);
export const InboxIcon = hi(_InboxIcon);
export const InformationCircleIcon = hi(_InformationCircleIcon);
export const KanbanIcon = hi(_KanbanIcon);
export const Key01Icon = hi(_Key01Icon);
export const LayoutGridIcon = hi(_LayoutGridIcon);
export const LayoutTable01Icon = hi(_LayoutTable01Icon);
export const LayoutTwoColumnIcon = hi(_LayoutTwoColumnIcon);
export const LeftToRightListBulletIcon = hi(_LeftToRightListBulletIcon);
export const LeftToRightListNumberIcon = hi(_LeftToRightListNumberIcon);
export const LifebuoyIcon = hi(_LifebuoyIcon);
export const Link01Icon = hi(_Link01Icon);
export const LinkSquare01Icon = hi(_LinkSquare01Icon);
export const Loading01Icon: IconComponent = ({ className, style }) => (
  <svg className={className} style={style} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray="50 20" />
  </svg>
);
export const Loading03Icon = Loading01Icon;
export const Logout01Icon = hi(_Logout01Icon);
export const MagicWand01Icon = hi(_MagicWand01Icon);
export const Mail01Icon = hi(_Mail01Icon);
export const MailAdd01Icon = hi(_MailAdd01Icon);
export const MailOpenIcon = hi(_MailOpenIcon);
export const MapPinIcon = hi(_MapPinIcon);
export const Maximize01Icon = hi(_Maximize01Icon);
export const Menu01Icon = hi(_Menu01Icon);
export const Message01Icon = hi(_Message01Icon);
export const MessagePreview01Icon = hi(_MessagePreview01Icon);
export const MessageCircleReplyIcon = hi(_MessageCircleReplyIcon);
export const MailReply01Icon = hi(_MailReply01Icon);
export const Minimize01Icon = hi(_Minimize01Icon);
export const Moon02Icon = hi(_Moon02Icon);
export const MoreHorizontalIcon = hi(_MoreHorizontalIcon);
export const MoreVerticalIcon = hi(_MoreVerticalIcon);
export const Notification02Icon = hi(_Notification02Icon);
export const NotificationBubbleIcon = hi(_NotificationBubbleIcon);
export const NotificationOff02Icon = hi(_NotificationOff02Icon);
export const PaintBoardIcon = hi(_PaintBoardIcon);
export const PauseIcon = hi(_PauseIcon);
export const PencilEdit01Icon = hi(_PencilEdit01Icon);
export const PencilEdit02Icon = hi(_PencilEdit02Icon);
export const PlayCircleIcon = hi(_PlayCircleIcon);
export const PlayIcon = hi(_PlayIcon);
export const PlusSignCircleIcon = hi(_PlusSignCircleIcon);
export const PlusSignIcon = hi(_PlusSignIcon);
export const ReplaceIcon = hi(_ReplaceIcon);
export const RotateLeft01Icon = hi(_RotateLeft01Icon);
export const Search01Icon = hi(_Search01Icon);
export const SecurityCheckIcon = hi(_SecurityCheckIcon);
export const SentIcon = hi(_SentIcon);
export const Setting06Icon = hi(_Setting06Icon);
export const Setting07Icon = hi(_Setting07Icon);
export const Settings02Icon = hi(_Settings02Icon);
export const Shield01Icon = hi(_Shield01Icon);
export const Shield02Icon = hi(_Shield02Icon);
export const SmileIcon = hi(_SmileIcon);
export const SmilePlusIcon = hi(_SmilePlusIcon);
export const SourceCodeIcon = hi(_SourceCodeIcon);
export const SparklesIcon = hi(_SparklesIcon);
export const AiMagicIcon = hi(_AiMagicIcon);
export const AiNetworkIcon = hi(_AiNetworkIcon);
export const CloudServerIcon = hi(_CloudServerIcon);
export const Unlink01Icon = hi(_Unlink01Icon);
export const StarIcon = hi(_StarIcon);
export const StickyNote01Icon = hi(_StickyNote01Icon);
export const Sun01Icon = hi(_Sun01Icon);
export const Tag01Icon = hi(_Tag01Icon);
export const Target01Icon = hi(_Target01Icon);
export const TelephoneIcon = hi(_TelephoneIcon);
export const TerminalIcon = hi(_TerminalIcon);
export const Tick01Icon = hi(_Tick01Icon);
export const TickDouble01Icon = hi(_TickDouble01Icon);
export const Timer01Icon = hi(_Timer01Icon);
export const Upload01Icon = hi(_Upload01Icon);
export const UserAdd01Icon = hi(_UserAdd01Icon);
export const UserGroupIcon = hi(_UserGroupIcon);
export const UserIcon = hi(_UserIcon);
export const ViewIcon = hi(_ViewIcon);
export const ViewOffIcon = hi(_ViewOffIcon);
export const WorkflowSquare01Icon = hi(_WorkflowSquare01Icon);
export const ZapIcon = hi(_ZapIcon);
export const ZoomInAreaIcon = hi(_ZoomInAreaIcon);
export const ZoomOutAreaIcon = hi(_ZoomOutAreaIcon);

export const Mic01Icon = hi(_Mic01Icon);
