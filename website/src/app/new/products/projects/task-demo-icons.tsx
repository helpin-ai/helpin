// TaskDetailPanel uses these Hugeicons paths (core-free-icons 4.1.1, MIT).
// Snapshot only the icons needed by this isolated marketing illustration.
import { createElement } from 'react';
const icons = {
  "CheckListIcon": [
    [
      "path",
      {
        "d": "M11 6L21 6",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M11 12L21 12",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M11 18L21 18",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "path",
      {
        "d": "M3 7.39286C3 7.39286 4 8.04466 4.5 9C4.5 9 6 5.25 8 4",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "3"
      }
    ],
    [
      "path",
      {
        "d": "M3 18.3929C3 18.3929 4 19.0447 4.5 20C4.5 20 6 16.25 8 15",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "4"
      }
    ]
  ],
  "Layers01Icon": [
    [
      "path",
      {
        "d": "M8.64298 3.14559L6.93816 3.93362C4.31272 5.14719 3 5.75397 3 6.75C3 7.74603 4.31272 8.35281 6.93817 9.56638L8.64298 10.3544C10.2952 11.1181 11.1214 11.5 12 11.5C12.8786 11.5 13.7048 11.1181 15.357 10.3544L17.0618 9.56638C19.6873 8.35281 21 7.74603 21 6.75C21 5.75397 19.6873 5.14719 17.0618 3.93362L15.357 3.14559C13.7048 2.38186 12.8786 2 12 2C11.1214 2 10.2952 2.38186 8.64298 3.14559Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M20.788 11.0972C20.9293 11.2959 21 11.5031 21 11.7309C21 12.7127 19.6873 13.3109 17.0618 14.5072L15.357 15.284C13.7048 16.0368 12.8786 16.4133 12 16.4133C11.1214 16.4133 10.2952 16.0368 8.64298 15.284L6.93817 14.5072C4.31272 13.3109 3 12.7127 3 11.7309C3 11.5031 3.07067 11.2959 3.212 11.0972",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M20.3767 16.2661C20.7922 16.5971 21 16.927 21 17.3176C21 18.2995 19.6873 18.8976 17.0618 20.0939L15.357 20.8707C13.7048 21.6236 12.8786 22 12 22C11.1214 22 10.2952 21.6236 8.64298 20.8707L6.93817 20.0939C4.31272 18.8976 3 18.2995 3 17.3176C3 16.927 3.20778 16.5971 3.62334 16.2661",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ]
  ],
  "ArrowRight01Icon": [
    [
      "path",
      {
        "d": "M9.00005 6C9.00005 6 15 10.4189 15 12C15 13.5812 9 18 9 18",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ]
  ],
  "Link01Icon": [
    [
      "path",
      {
        "d": "M9.14339 10.691L9.35031 10.4841C11.329 8.50532 14.5372 8.50532 16.5159 10.4841C18.4947 12.4628 18.4947 15.671 16.5159 17.6497L13.6497 20.5159C11.671 22.4947 8.46279 22.4947 6.48405 20.5159C4.50532 18.5372 4.50532 15.329 6.48405 13.3503L6.9484 12.886",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M17.0516 11.114L17.5159 10.6497C19.4947 8.67095 19.4947 5.46279 17.5159 3.48405C15.5372 1.50532 12.329 1.50532 10.3503 3.48405L7.48405 6.35031C5.50532 8.32904 5.50532 11.5372 7.48405 13.5159C9.46279 15.4947 12.671 15.4947 14.6497 13.5159L14.8566 13.309",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "MoreVerticalIcon": [
    [
      "path",
      {
        "d": "M11.9967 12.5V12M11.9967 6.5V6M11.9967 18.5V18M12.9967 12.5C12.9967 11.9477 12.549 11.5 11.9967 11.5C11.4444 11.5 10.9967 11.9477 10.9967 12.5C10.9967 13.0523 11.4444 13.5 11.9967 13.5C12.549 13.5 12.9967 13.0523 12.9967 12.5ZM12.9967 6.5C12.9967 5.94772 12.549 5.5 11.9967 5.5C11.4444 5.5 10.9967 5.94772 10.9967 6.5C10.9967 7.05228 11.4444 7.5 11.9967 7.5C12.549 7.5 12.9967 7.05228 12.9967 6.5ZM12.9967 18.5C12.9967 17.9477 12.549 17.5 11.9967 17.5C11.4444 17.5 10.9967 17.9477 10.9967 18.5C10.9967 19.0523 11.4444 19.5 11.9967 19.5C12.549 19.5 12.9967 19.0523 12.9967 18.5Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ]
  ],
  "ArrowUpRight01Icon": [
    [
      "path",
      {
        "d": "M9 6.65032C9 6.65032 15.9383 6.10759 16.9154 7.08463C17.8924 8.06167 17.3496 15 17.3496 15M16.5 7.5L6.5 17.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ]
  ],
  "Cancel01Icon": [
    [
      "path",
      {
        "d": "M18 6L6.00081 17.9992M17.9992 18L6 6.00085",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ]
  ],
  "Copy01Icon": [
    [
      "path",
      {
        "d": "M9 15C9 12.1716 9 10.7574 9.87868 9.87868C10.7574 9 12.1716 9 15 9L16 9C18.8284 9 20.2426 9 21.1213 9.87868C22 10.7574 22 12.1716 22 15V16C22 18.8284 22 20.2426 21.1213 21.1213C20.2426 22 18.8284 22 16 22H15C12.1716 22 10.7574 22 9.87868 21.1213C9 20.2426 9 18.8284 9 16L9 15Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M16.9999 9C16.9975 6.04291 16.9528 4.51121 16.092 3.46243C15.9258 3.25989 15.7401 3.07418 15.5376 2.90796C14.4312 2 12.7875 2 9.5 2C6.21252 2 4.56878 2 3.46243 2.90796C3.25989 3.07417 3.07418 3.25989 2.90796 3.46243C2 4.56878 2 6.21252 2 9.5C2 12.7875 2 14.4312 2.90796 15.5376C3.07417 15.7401 3.25989 15.9258 3.46243 16.092C4.51121 16.9528 6.04291 16.9975 9 16.9999",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "GitBranchIcon": [
    [
      "path",
      {
        "d": "M7 19H13C15.8284 19 17.2426 19 18.1213 18.1213C19 17.2426 19 15.8284 19 13V10M19 10C19.7002 10 21.0085 11.9943 21.5 12.5M19 10C18.2998 10 16.9915 11.9943 16.5 12.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M5 7L5 17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "circle",
      {
        "cx": "5",
        "cy": "5",
        "r": "2",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "circle",
      {
        "cx": "19",
        "cy": "5",
        "r": "2",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "3"
      }
    ],
    [
      "circle",
      {
        "cx": "5",
        "cy": "19",
        "r": "2",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "4"
      }
    ]
  ],
  "UserGroupIcon": [
    [
      "path",
      {
        "d": "M15.5 11C15.5 9.067 13.933 7.5 12 7.5C10.067 7.5 8.5 9.067 8.5 11C8.5 12.933 10.067 14.5 12 14.5C13.933 14.5 15.5 12.933 15.5 11Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M15.4827 11.3499C15.8047 11.4475 16.1462 11.5 16.5 11.5C18.433 11.5 20 9.933 20 8C20 6.067 18.433 4.5 16.5 4.5C14.6851 4.5 13.1928 5.8814 13.0173 7.65013",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M10.9827 7.65013C10.8072 5.8814 9.31492 4.5 7.5 4.5C5.567 4.5 4 6.067 4 8C4 9.933 5.567 11.5 7.5 11.5C7.85381 11.5 8.19535 11.4475 8.51727 11.3499",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "path",
      {
        "d": "M22 16.5C22 13.7386 19.5376 11.5 16.5 11.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "3"
      }
    ],
    [
      "path",
      {
        "d": "M17.5 19.5C17.5 16.7386 15.0376 14.5 12 14.5C8.96243 14.5 6.5 16.7386 6.5 19.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "4"
      }
    ],
    [
      "path",
      {
        "d": "M7.5 11.5C4.46243 11.5 2 13.7386 2 16.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "5"
      }
    ]
  ],
  "HashtagIcon": [
    [
      "path",
      {
        "d": "M14 21L18 3",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M6 21L10 3",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M5 8H21",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "path",
      {
        "d": "M3 16H19",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "3"
      }
    ]
  ],
  "UserIcon": [
    [
      "path",
      {
        "d": "M17 8.5C17 5.73858 14.7614 3.5 12 3.5C9.23858 3.5 7 5.73858 7 8.5C7 11.2614 9.23858 13.5 12 13.5C14.7614 13.5 17 11.2614 17 8.5Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M19 20.5C19 16.634 15.866 13.5 12 13.5C8.13401 13.5 5 16.634 5 20.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "DashboardSpeed01Icon": [
    [
      "path",
      {
        "d": "M13.5 13L17 9M14 15C14 16.1046 13.1046 17 12 17C10.8954 17 10 16.1046 10 15C10 13.8954 10.8954 13 12 13C13.1046 13 14 13.8954 14 15Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M6 12C6 8.68629 8.68629 6 12 6C13.0929 6 14.1175 6.29218 15 6.80269",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M2.50006 12.0001C2.50006 7.52172 2.50006 5.28255 3.8913 3.8913C5.28255 2.50006 7.52172 2.50006 12.0001 2.50006C16.4784 2.50006 18.7176 2.50006 20.1088 3.8913C21.5001 5.28255 21.5001 7.52172 21.5001 12.0001C21.5001 16.4784 21.5001 18.7176 20.1088 20.1088C18.7176 21.5001 16.4784 21.5001 12.0001 21.5001C7.52172 21.5001 5.28255 21.5001 3.8913 20.1088C2.50006 18.7176 2.50006 16.4784 2.50006 12.0001Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "2"
      }
    ]
  ],
  "Tag01Icon": [
    [
      "circle",
      {
        "cx": "1.5",
        "cy": "1.5",
        "r": "1.5",
        "transform": "matrix(1 0 0 -1 16 8.00024)",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M2.77423 11.1439C1.77108 12.2643 1.7495 13.9546 2.67016 15.1437C4.49711 17.5033 6.49674 19.5029 8.85633 21.3298C10.0454 22.2505 11.7357 22.2289 12.8561 21.2258C15.8979 18.5022 18.6835 15.6559 21.3719 12.5279C21.6377 12.2187 21.8039 11.8397 21.8412 11.4336C22.0062 9.63798 22.3452 4.46467 20.9403 3.05974C19.5353 1.65481 14.362 1.99377 12.5664 2.15876C12.1603 2.19608 11.7813 2.36233 11.472 2.62811C8.34412 5.31646 5.49781 8.10211 2.77423 11.1439Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M7.00002 14.0002L10 17.0002",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ]
  ],
  "LayoutGridIcon": [
    [
      "path",
      {
        "d": "M20.1088 3.89124C21.5 5.28249 21.5 7.52166 21.5 12C21.5 16.4783 21.5 18.7175 20.1088 20.1088C18.7175 21.5 16.4783 21.5 12 21.5C7.52166 21.5 5.28249 21.5 3.89124 20.1088C2.5 18.7175 2.5 16.4783 2.5 12C2.5 7.52166 2.5 5.28249 3.89124 3.89124C5.28249 2.5 7.52166 2.5 12 2.5C16.4783 2.5 18.7175 2.5 20.1088 3.89124Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M21.5 12L2.5 12",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M12 2.5L12 21.5",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ]
  ],
  "PencilEdit01Icon": [
    [
      "path",
      {
        "d": "M15.2141 5.98239L16.6158 4.58063C17.39 3.80646 18.6452 3.80646 19.4194 4.58063C20.1935 5.3548 20.1935 6.60998 19.4194 7.38415L18.0176 8.78591M15.2141 5.98239L6.98023 14.2163C5.93493 15.2616 5.41226 15.7842 5.05637 16.4211C4.70047 17.058 4.3424 18.5619 4 20C5.43809 19.6576 6.94199 19.2995 7.57889 18.9436C8.21579 18.5877 8.73844 18.0651 9.78375 17.0198L18.0176 8.78591M15.2141 5.98239L18.0176 8.78591",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M11 20H17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "PlusSignIcon": [
    [
      "path",
      {
        "d": "M12 4V20M20 12H4",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ]
  ],
  "FilterHorizontalIcon": [
    [
      "path",
      {
        "d": "M3 7H6",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M3 17H9",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M18 17L21 17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "path",
      {
        "d": "M15 7L21 7",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "3"
      }
    ],
    [
      "path",
      {
        "d": "M6 7C6 6.06812 6 5.60218 6.15224 5.23463C6.35523 4.74458 6.74458 4.35523 7.23463 4.15224C7.60218 4 8.06812 4 9 4C9.93188 4 10.3978 4 10.7654 4.15224C11.2554 4.35523 11.6448 4.74458 11.8478 5.23463C12 5.60218 12 6.06812 12 7C12 7.93188 12 8.39782 11.8478 8.76537C11.6448 9.25542 11.2554 9.64477 10.7654 9.84776C10.3978 10 9.93188 10 9 10C8.06812 10 7.60218 10 7.23463 9.84776C6.74458 9.64477 6.35523 9.25542 6.15224 8.76537C6 8.39782 6 7.93188 6 7Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "4"
      }
    ],
    [
      "path",
      {
        "d": "M12 17C12 16.0681 12 15.6022 12.1522 15.2346C12.3552 14.7446 12.7446 14.3552 13.2346 14.1522C13.6022 14 14.0681 14 15 14C15.9319 14 16.3978 14 16.7654 14.1522C17.2554 14.3552 17.6448 14.7446 17.8478 15.2346C18 15.6022 18 16.0681 18 17C18 17.9319 18 18.3978 17.8478 18.7654C17.6448 19.2554 17.2554 19.6448 16.7654 19.8478C16.3978 20 15.9319 20 15 20C14.0681 20 13.6022 20 13.2346 19.8478C12.7446 19.6448 12.3552 19.2554 12.1522 18.7654C12 18.3978 12 17.9319 12 17Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "5"
      }
    ]
  ],
  "Search01Icon": [
    [
      "path",
      {
        "d": "M17 17L21 21",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M19 11C19 6.58172 15.4183 3 11 3C6.58172 3 3 6.58172 3 11C3 15.4183 6.58172 19 11 19C15.4183 19 19 15.4183 19 11Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "ViewIcon": [
    [
      "path",
      {
        "d": "M21.544 11.045C21.848 11.4713 22 11.6845 22 12C22 12.3155 21.848 12.5287 21.544 12.955C20.1779 14.8706 16.6892 19 12 19C7.31078 19 3.8221 14.8706 2.45604 12.955C2.15201 12.5287 2 12.3155 2 12C2 11.6845 2.15201 11.4713 2.45604 11.045C3.8221 9.12944 7.31078 5 12 5C16.6892 5 20.1779 9.12944 21.544 11.045Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15C13.6569 15 15 13.6569 15 12Z",
        "stroke": "currentColor",
        "strokeWidth": 2,
        "key": "1"
      }
    ]
  ],
  "File01Icon": [
    [
      "path",
      {
        "d": "M8 7L16 7",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M8 11L12 11",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M13 21.5V21C13 18.1716 13 16.7574 13.8787 15.8787C14.7574 15 16.1716 15 19 15H19.5M20 13.3431V10C20 6.22876 20 4.34315 18.8284 3.17157C17.6569 2 15.7712 2 12 2C8.22877 2 6.34315 2 5.17157 3.17157C4 4.34314 4 6.22876 4 10L4 14.5442C4 17.7892 4 19.4117 4.88607 20.5107C5.06508 20.7327 5.26731 20.9349 5.48933 21.1139C6.58831 22 8.21082 22 11.4558 22C12.1614 22 12.5141 22 12.8372 21.886C12.9044 21.8623 12.9702 21.835 13.0345 21.8043C13.3436 21.6564 13.593 21.407 14.0919 20.9081L18.8284 16.1716C19.4065 15.5935 19.6955 15.3045 19.8478 14.9369C20 14.5694 20 14.1606 20 13.3431Z",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ]
  ],
  "ChartColumnIcon": [
    [
      "path",
      {
        "d": "M8 9V17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "0"
      }
    ],
    [
      "path",
      {
        "d": "M13 5V17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "1"
      }
    ],
    [
      "path",
      {
        "d": "M18 13V17",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "2"
      }
    ],
    [
      "path",
      {
        "d": "M3 3V13C3 16.7712 3 18.6569 4.17157 19.8284C5.34315 21 7.22876 21 11 21H21",
        "stroke": "currentColor",
        "strokeLinecap": "round",
        "strokeLinejoin": "round",
        "strokeWidth": 2,
        "key": "3"
      }
    ]
  ]
};
export type TaskIconName = keyof typeof icons;
export function TaskIcon({ name }: { name: TaskIconName }) {
 return <svg viewBox="0 0 24 24" fill="none" width={14} height={14} aria-hidden="true">{icons[name].map(([tag, attrs], i) => createElement(tag as string, { ...(attrs as Record<string, unknown>), key: i }))}</svg>;
}

// Exact task-specific geometry from frontend/src/lib/pmIcons.tsx.
export function TaskSprintIcon() {
 return <svg viewBox="0 0 24 24" fill="none" width={14} height={14} aria-hidden="true" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M7 7H17L14.5 4.5"/><path d="M17 17H7L9.5 19.5"/><path d="M17 7C19.2091 7 21 8.79086 21 11"/><path d="M7 17C4.79086 17 3 15.2091 3 13"/></svg>;
}
export function TaskFeatureIcon() {
 return <svg viewBox="0 0 24 24" width={14} height={14} fill="currentColor" aria-hidden="true" className="pth-feature"><g transform="translate(12 12) scale(1.08) translate(-12 -12)"><path d="M12 4L14 10L20 12L14 14L12 20L10 14L4 12L10 10Z"/><path d="M20 3L20.5 4.5L22 5L20.5 5.5L20 7L19.5 5.5L18 5L19.5 4.5Z"/><path d="M4 17L4.5 18.5L6 19L4.5 19.5L4 21L3.5 19.5L2 19L3.5 18.5Z"/></g></svg>;
}
export function TaskPriorityIcon() {
 return <svg viewBox="0 0 24 24" width={14} height={14} fill="currentColor" aria-hidden="true" className="pth-priority"><rect x="5" y="13" width="3" height="6" rx="1"/><rect x="10.5" y="10" width="3" height="9" rx="1"/><rect x="16" y="7" width="3" height="12" rx="1"/></svg>;
}
export function TaskTickIcon() {
 return <svg viewBox="0 0 24 24" width={12} height={12} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M5 12.5L9.5 17L19 7.5"/></svg>;
}
export function TaskCalendarIcon() {
 return <svg viewBox="0 0 24 24" width={14} height={14} fill="none" stroke="currentColor" strokeWidth={2} aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2.5"/><path d="M8 3V7M16 3V7" strokeLinecap="round"/><path d="M3 10H21"/></svg>;
}
