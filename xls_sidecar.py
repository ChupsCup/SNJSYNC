
"""Baca Excel .xls lama (BIFF8/OLE2) lalu keluarkan JSON ke stdout.

excelize (Go) hanya bisa membuka .xlsx (OOXML). File .xls sungguhan milik user
bukan zip, jadi harus dibaca pustaka lain. xlrd 2.x memang khusus .xls/BIFF dan
sudah jadi pembaca paling tearing untuk format ini.

Pakai:
    python xls_sidecar.py <path-file.xls>

Output:
    {"sheets": [{"name": "...", "rows": [[{"s": "..", "num": true}, ...], ...]}]}

Cell dikembalikan sebagai objek {"s": <teks>, "num": <bool>} supaya Go bisa
membangun cellVal tanpa menebak tipe lagi. Nilai numerik floating point
dibulatkan seperti Excel (15 digit signifikan) supaya tidak muncul seperti
1.0000000000000002 di UI.
"""

import json
import sys


XL_CELL_EMPTY = 0
XL_CELL_TEXT = 1
XL_CELL_NUMBER = 2
XL_CELL_DATE = 3
XL_CELL_BOOLEAN = 4
XL_CELL_ERROR = 5
XL_CELL_BLANK = 6


def fmt_number(v):
    """Format angka float seperti Excel: tanpa eksponensial, tanpa noise float."""
    if v == int(v) and abs(v) < 1e15:
        return str(int(v))
    out = "%.15g" % v
    return out


def cell(value, ctype):
    
    if ctype == XL_CELL_EMPTY or ctype == XL_CELL_BLANK or value is None or value == "":
        return {"s": "", "num": False}
    if ctype == XL_CELL_NUMBER:
        if isinstance(value, float):
            s = fmt_number(value)
        else:
            s = str(value)
        return {"s": s, "num": True}
    if ctype == XL_CELL_DATE:
        
        
        return {"s": fmt_number(value) if isinstance(value, float) else str(value), "num": True}
    if ctype == XL_CELL_BOOLEAN:
        return {"s": "TRUE" if value else "FALSE", "num": False}
    if ctype == XL_CELL_ERROR:
        return {"s": "
    
    return {"s": str(value), "num": False}


def main():
    
    
    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    except AttributeError:
        pass

    if len(sys.argv) < 2:
        print(json.dumps({"error": "path file .xls wajib diberikan"}))
        return 2
    path = sys.argv[1]

    try:
        import xlrd
    except ImportError:
        print(json.dumps({
            "error": "xlrd tidak terpasang. Jalankan: pip install xlrd==2.0.2"
        }))
        return 3

    try:
        book = xlrd.open_workbook(path, formatting_info=False, on_demand=True)
    except xlrd.XLRDError as exc:
        msg = str(exc)
        if "password" in msg.lower() or "encrypt" in msg.lower():
            msg = ("file .xls ini terenkripsi/password. Buka di Excel, "
                   "hapus password, lalu simpan ulang.")
        print(json.dumps({"error": msg}))
        return 4
    except Exception as exc:  
        print(json.dumps({"error": "%s: %s" % (type(exc).__name__, exc)}))
        return 5

    sheets = []
    try:
        for name in book.sheet_names():
            sh = book.sheet_by_name(name)
            rows = []
            for r in range(sh.nrows):
                row = []
                for c in range(sh.ncols):
                    ctype = sh.cell_type(r, c)
                    value = sh.cell_value(r, c)
                    row.append(cell(value, ctype))
                rows.append(row)
            sheets.append({"name": name, "rows": rows})
    finally:
        try:
            book.release_resources()
        except Exception:  
            pass

    json.dump({"sheets": sheets}, sys.stdout, ensure_ascii=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
