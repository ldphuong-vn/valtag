from setuptools import setup, find_packages

setup(
    name="valtag",
    version="0.1.0",
    packages=find_packages(),
    python_requires=">=3.8",
    description="Valtag secret vault Python SDK",
    long_description="A minimal Python client for the Valtag secret vault API.",
    author="valtag",
    license="MIT",
    package_data={"valtag": ["py.typed"]},
)
