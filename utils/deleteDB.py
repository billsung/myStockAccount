import sqlite3

def type1():
    # 查找符合條件的表格名稱
    cursor.execute("SELECT name FROM sqlite_master WHERE type='table' AND name GLOB 'stk7[0-9]*U';")
    tables = cursor.fetchall()

    # 關閉外鍵約束（避免外鍵錯誤）
    cursor.execute("PRAGMA foreign_keys = OFF;")

    # 逐個刪除表
    for table in tables:
        table_name = table[0]
        print(f"Dropping table: {table_name}")
        cursor.execute(f"DROP TABLE {table_name};")

    # 重新啟用外鍵約束
    cursor.execute("PRAGMA foreign_keys = ON;")

    # 儲存變更
    conn.commit()
    conn.close()

    print("Type1 matching tables have been dropped.")

def type2():
    # 查找符合條件的表格名稱
    cursor.execute("""
        SELECT name FROM sqlite_master 
        WHERE type='table' 
        AND name GLOB 'stk7[0-9]*'
        AND (LENGTH(REPLACE(name, 'stk', '')) - LENGTH(REPLACE(REPLACE(name, 'stk', ''), '7', ''))) >= 1
        AND LENGTH(name) = 9;
    """)
    tables = cursor.fetchall()

    # 關閉外鍵約束
    cursor.execute("PRAGMA foreign_keys = OFF;")

    # 逐個刪除表
    for table in tables:
        table_name = table[0]
        print(f"Dropping table: {table_name}")
        cursor.execute(f"DROP TABLE {table_name};")

    # 重新啟用外鍵約束
    cursor.execute("PRAGMA foreign_keys = ON;")

    # 儲存變更
    conn.commit()
    conn.close()

    print("Type2 matching tables have been dropped.")

# 連接到 SQLite 資料庫
db_path = "..\database\dailyDB.sqlite"
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

type2()




