import re

with open('pkg/backup/backup.go', 'r') as f:
    content = f.read()

content = content.replace("""			}
			logger.Info("backup action completed", "name", msg, "kind", act.GetKind())""", """			}

			logger.Info("backup action completed", "name", msg, "kind", act.GetKind())""")

with open('pkg/backup/backup.go', 'w') as f:
    f.write(content)
